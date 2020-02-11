// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package firestore_backend

import (
	"cloud.google.com/go/firestore"
	"context"
	"fmt"
	"github.com/benbjohnson/clock"
	"github.com/hchauvin/name_manager/pkg/internal/hold"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"google.golang.org/api/iterator"
	"strconv"
	"time"
)

var backendDescription = `Firestore backend.

The Firestore backend implements the name manager on top of Google's Cloud Firestore.
`

func init() {
	name_manager.RegisterBackend(name_manager.Backend{
		Protocol:          "firestore",
		Description:       backendDescription,
		CreateNameManager: createNameManager,
	})
}

func createNameManager(backendURL string) (name_manager.NameManager, error) {
	options, err := parseBackendURL(backendURL)
	if err != nil {
		return nil, err
	}
	return &firestoreBackend{
		options: *options,
	}, nil
}

// firestoreBackend implements the Firestore backend.
//
// The database is comprised of the following documents:
// - "families/{families}" (of type familyData): global info on families.
// - "families/{families}/names/{names}" (of type nameData): one entry per name in a family.
//
// The CreatedAt/UpdatedAt fields come directly from Firestore.
type firestoreBackend struct {
	// options are the options for the backend.
	options options
}

// familyData contains the data that goes in "families/{family}" documents
type familyData struct {
	// Count contains the number of names for the family.
	Count int `firestore:"count"`
}

// nameData contains the data that goes in "names/{family}/{name}" documents.
type nameData struct {
	// Free is true if the name was acquired in the past but is now free.
	Free bool `firestore:"free"`
}

func (fbk *firestoreBackend) Hold(family string) (string, <-chan error, name_manager.ReleaseFunc, error) {
	return fbk.hold().Hold(family)
}

func (fbk *firestoreBackend) Acquire(family string) (string, error) {
	ctx := context.Background()

	client, err := fbk.client()
	if err != nil {
		return "", err
	}
	defer client.Close()

	// We cannot release the zombies in the transaction as listing does not work in transactions
	// (at least in Firestore emulators).
	if autoReleaseAfter := fbk.options.autoReleaseAfter; autoReleaseAfter > 0 {
		if err := releaseZombies(ctx, client, autoReleaseAfter, family); err != nil {
			return "", err
		}
	}

	name := ""
	if err := client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// Try to get the first free name
		nameDoc, err := tx.Documents(client.Collection("families/"+family+"/names").
			Where("free", "==", true).
			Limit(1)).Next()
		if err == nil {
			// A free name could be found
			name = nameDoc.Ref.ID

			// Acquire the free name
			if err := tx.Set(nameDoc.Ref, nameData{Free: false}); err != nil {
				return err
			}
			return nil
		}

		if err != iterator.Done {
			// There was an error during the search for a free name
			return err
		}

		// No free name could be found: a new name will be created using the
		// counter stored at the family level.

		familyRef := client.Doc("families/" + family)

		familyDoc, err := txGet(tx, familyRef)
		if err != nil {
			return err
		}
		familyD := familyData{}
		if familyDoc.Exists() {
			if err := familyDoc.DataTo(&familyD); err != nil {
				return err
			}
		}
		name = strconv.Itoa(familyD.Count)

		if err := tx.Set(
			client.Doc(fmt.Sprintf("families/%s/names/%s", family, name)),
			nameData{Free: false},
		); err != nil {
			return err
		}

		familyD.Count += 1
		err = tx.Set(familyRef, familyD)
		return err
	}); err != nil {
		return "", err
	}
	return name, nil
}

func (fbk *firestoreBackend) KeepAlive(family, name string) error {
	ctx := context.Background()

	client, err := fbk.client()
	if err != nil {
		return err
	}
	defer client.Close()

	return client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		nameRef := client.Doc(fmt.Sprintf("families/%s/names/%s", family, name))
		doc, err := txGet(tx, nameRef)
		if err != nil {
			return err
		}

		if !doc.Exists() {
			return nil
		}
		nameD := nameData{}
		if err := doc.DataTo(&nameD); err != nil {
			return err
		}
		if nameD.Free {
			return nil
		}

		err = tx.Set(nameRef, nameD)
		return err
	})
}

func (fbk *firestoreBackend) Release(family, name string) error {
	ctx := context.Background()

	client, err := fbk.client()
	if err != nil {
		return err
	}
	defer client.Close()

	return client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		return release(client, tx, family, name)
	})
}

func (fbk *firestoreBackend) TryHold(family, name string) (<-chan error, name_manager.ReleaseFunc, error) {
	return fbk.hold().TryHold(family, name)
}

func (fbk *firestoreBackend) TryAcquire(family, name string) error {
	ctx := context.Background()

	client, err := fbk.client()
	if err != nil {
		return err
	}
	defer client.Close()

	return client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		nameRef := client.Doc(fmt.Sprintf("families/%s/names/%s", family, name))
		nameDoc, err := txGet(tx, nameRef)
		if err != nil {
			return err
		}
		if !nameDoc.Exists() {
			return name_manager.ErrNotExist
		}
		nameD := nameData{}
		if err := nameDoc.DataTo(&nameD); err != nil {
			return err
		}
		if !nameD.Free {
			return name_manager.ErrInUse
		}
		nameD.Free = false
		return tx.Set(nameRef, nameD)
	})
}

func (fbk *firestoreBackend) List() ([]name_manager.Name, error) {
	ctx := context.Background()

	client, err := fbk.client()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var names []name_manager.Name
	familyIter := client.Collection("families").Documents(ctx)
	for {
		familyDoc, err := familyIter.Next()
		if err != nil {
			if err == iterator.Done {
				break
			}
			return nil, err
		}

		nameIter := familyDoc.Ref.Collection("names").Documents(ctx)
		for {
			nameDoc, err := nameIter.Next()
			if err != nil {
				if err == iterator.Done {
					break
				}
				return nil, err
			}

			nameD := nameData{}
			if err := nameDoc.DataTo(&nameD); err != nil {
				return nil, err
			}

			names = append(names, name_manager.Name{
				Name:      nameDoc.Ref.ID,
				Family:    familyDoc.Ref.ID,
				CreatedAt: nameDoc.CreateTime,
				UpdatedAt: nameDoc.UpdateTime,
				Free:      nameD.Free,
			})
		}
	}
	return names, nil
}

func (fbk *firestoreBackend) Reset() error {
	ctx := context.Background()

	client, err := fbk.client()
	if err != nil {
		return err
	}
	defer client.Close()

	familyIter := client.Collection("families").Documents(ctx)
	for {
		familyDoc, err := familyIter.Next()
		if err != nil {
			if err == iterator.Done {
				break
			}
			return err
		}

		nameIter := familyDoc.Ref.Collection("names").Documents(ctx)
		for {
			nameDoc, err := nameIter.Next()
			if err != nil {
				if err == iterator.Done {
					break
				}
				return err
			}

			if _, err := nameDoc.Ref.Delete(ctx); err != nil {
				return err
			}
		}

		if _, err := familyDoc.Ref.Delete(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (fbk *firestoreBackend) client() (*firestore.Client, error) {
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelConnect()
	return firestore.NewClient(connectCtx, fbk.options.projectID)
}

func (fbk *firestoreBackend) hold() *hold.Hold {
	return &hold.Hold{
		Manager:           fbk,
		Clock:             clock.New(),
		KeepAliveInterval: fbk.options.autoReleaseAfter / 3,
	}
}

// txGet, contrary to tx.Get, does not error when the document does not exist.
func txGet(tx *firestore.Transaction, dr *firestore.DocumentRef) (*firestore.DocumentSnapshot, error) {
	docs, err := tx.GetAll([]*firestore.DocumentRef{dr})
	if err != nil {
		return nil, err
	}
	return docs[0], nil
}

// release implements name release inside a Firestore transaction.
func release(client *firestore.Client, tx *firestore.Transaction, family, name string) error {
	nameRef := client.Doc(fmt.Sprintf("families/%s/names/%s", family, name))
	nameDoc, err := txGet(tx, nameRef)
	if err != nil {
		return err
	}
	// We only add a free name if the name has some data associated to
	// it.
	if !nameDoc.Exists() {
		return nil
	}
	return tx.Set(nameRef, nameData{Free: true})
}

// releaseZombies releases the zombie names (that is, those that are not kept alive anymore
// and should be garbage-collected).
func releaseZombies(ctx context.Context, client *firestore.Client, autoReleaseAfter time.Duration, family string) error {
	now := time.Now()
	familyIter := client.Collection("families").Documents(ctx)
	for {
		familyDoc, err := familyIter.Next()
		if err != nil {
			if err == iterator.Done {
				break
			}
			return err
		}

		nameIter := familyDoc.Ref.Collection("names").Documents(ctx)
		for {
			nameDoc, err := nameIter.Next()
			if err != nil {
				if err == iterator.Done {
					break
				}
				return err
			}

			if now.Sub(nameDoc.UpdateTime) > autoReleaseAfter {
				if err := client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
					return release(client, tx, family, nameDoc.Ref.ID)
				}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
