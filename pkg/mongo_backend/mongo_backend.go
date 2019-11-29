package mongo_backend

import (
	"context"
	"fmt"
	"github.com/benbjohnson/clock"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	mongo_options "go.mongodb.org/mongo-driver/mongo/options"
	"os"
	"strconv"
	"time"
)

var backendDescription = `MongoDB backend.

The Mongo backend implements the name manager on top of MongoDB.
`

func init() {
	name_manager.RegisterBackend(name_manager.Backend{
		Protocol:          "mongo",
		Description:       backendDescription,
		CreateNameManager: createNameManager,
	})
}

func createNameManager(backendURL string) (name_manager.NameManager, error) {
	options, err := parseBackendURL(backendURL)
	if err != nil {
		return nil, err
	}
	return &mongoBackend{
		clock:   clock.New(),
		options: *options,
	}, nil
}

type mongoBackend struct {
	// clock is the clock used to get the CreatedAt/UpdatedAt timestamps.
	clock clock.Clock

	// options are the options for the backend.
	options options
}

const (
	// dataBucket is the name of the Bolt bucket that contains the metadata
	// associated to a name.  In this bucket, there is one entry per name,
	// the keys have the format `<family>:<name>` and the values are
	// json-marshalled `localBackendData` objects.
	dataCollection = "data"

	// freeNames bucket is the name of the Bolt bucket that is used to
	// keep track of all the free names.  In this bucket, there is one
	// entry per free name, the keys have the format `<family>:<name>`
	// and the values are all set to the `freeValue` placeholder.
	leasedNamesCollection = "leasedNames"

	// countersCollection is the name of the MongoDB collection that is used to
	// keep track of the number of names for each family.
	countersCollection = "counters"
)

const lockDocumentPartition = "partition"

const mongoDBDuplicateKeyErrorCode = 11000

func (mbk *mongoBackend) Hold(family string) (string, name_manager.ReleaseFunc, error) {
	name, err := mbk.Acquire(family)
	if err != nil {
		return "", nil, err
	}

	stopKeepAlive := make(chan struct{})
	keepAliveDone := make(chan struct{})
	if mbk.options.autoReleaseAfter > 0 {
		go func() {
			for {
				select {
				case <-stopKeepAlive:
					keepAliveDone <- struct{}{}
					break
				case <-mbk.clock.After(mbk.options.autoReleaseAfter / 3):
				}

				if err := mbk.KeepAlive(family, name); err != nil {
					fmt.Fprintf(os.Stderr, "cannot keep alive %s:%s: %v\n", family, name, err)
					break
				}
			}
		}()
	}

	releaseFunc := func() error {
		if mbk.options.autoReleaseAfter > 0 {
			stopKeepAlive <- struct{}{}
			<-keepAliveDone
		}
		if err := mbk.Release(family, name); err != nil {
			return err
		}
		return nil
	}

	return name, releaseFunc, nil
}

func (mbk *mongoBackend) Acquire(family string) (string, error) {
	ctx := context.Background()

	client, err := mbk.client()
	if err != nil {
		return "", err
	}
	defer client.Disconnect(ctx)

	db := client.Database(mbk.options.database)

	if err := mbk.releaseZombies(ctx, db, family); err != nil {
		return "", err
	}

	result, err := mbk.collection(db, dataCollection).
		Find(context.Background(), bson.D{{"family", family}})
	if err != nil {
		return "", err
	}
	for result.Next(ctx) {
		if result.Err() != nil {
			return "", result.Err()
		}

		curName := result.Current.Lookup("name").StringValue()

		// Let's try to get a lease on this name.
		now := mbk.clock.Now()
		document := bson.M{
			"_id": mbk.leaseId(family, curName),
			// partition is used by CosmosDB.
			"partition":         "partition",
			"createdAt":         now,
			"lastHeartBeatDate": now,
			"family":            family,
		}
		_, err = mbk.collection(db, leasedNamesCollection).
			InsertOne(ctx, document)
		if err == nil {
			// The lease was successfully acquired
			return curName, nil
		}

		// The lease could not be acquired.  There is either a problem with the MongoDB
		// backend, or the name is already leased.
		if werrs, ok := err.(mongo.WriteException); ok {
			for _, werr := range werrs.WriteErrors {
				if werr.Code == mongoDBDuplicateKeyErrorCode {
					// The name is already leased, let's try another name.
					goto next
				}
			}
		}
		return "", err

	next:
	}

	// We looped through all the names, and we could not acquire any.
	// It means we need to create a new name.  To avoid having another
	// process acquiring the name we just created, we need to lease
	// the name *before* we actually create it.  We ensure the uniqueness
	// of the new name by atomically updating the family entry in the
	// counters collection.
	counterResult := mbk.collection(db, countersCollection).
		FindOneAndUpdate(
			ctx,
			bson.M{"family": family},
			bson.M{"$inc": bson.M{"counter": 1}},
			mongo_options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(mongo_options.Before))
	var counter int32
	if counterResult.Err() == mongo.ErrNoDocuments {
		counter = 0 // Redundant, but clearer
	} else if counterResult.Err() != nil {
		return "", err
	} else {
		counterDoc, err := counterResult.DecodeBytes()
		if err != nil {
			return "", err
		}
		counter = counterDoc.Lookup("counter").Int32()
	}

	newName := strconv.Itoa(int(counter))

	now := mbk.clock.Now()
	document := bson.M{
		"_id":               mbk.leaseId(family, newName),
		"partition":         lockDocumentPartition,
		"createdAt":         now,
		"lastHeartBeatDate": now,
		"family":            family,
	}
	_, err = mbk.collection(db, leasedNamesCollection).
		InsertOne(ctx, document)
	if err != nil {
		return "", err
	}

	document = bson.M{
		"family":    family,
		"name":      newName,
		"createdAt": now,
	}
	_, err = mbk.collection(db, dataCollection).
		InsertOne(ctx, document)
	if err != nil {
		return "", err
	}

	return newName, nil
}

func (mbk *mongoBackend) KeepAlive(family, name string) error {
	ctx := context.Background()

	client, err := mbk.client()
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)

	db := client.Database(mbk.options.database)

	_, err = mbk.collection(db, leasedNamesCollection).UpdateOne(
		context.Background(),
		bson.M{
			"_id":       mbk.leaseId(family, name),
			"partition": lockDocumentPartition,
		},
		bson.M{"$set": bson.M{"lastHeartBeatDate": mbk.clock.Now()}})
	return err
}

func (mbk *mongoBackend) Release(family, name string) error {
	ctx := context.Background()

	client, err := mbk.client()
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)

	db := client.Database(mbk.options.database)

	_, err = mbk.collection(db, leasedNamesCollection).
		DeleteOne(ctx, bson.D{{"_id", mbk.leaseId(family, name)}})
	return err
}

func (mbk *mongoBackend) TryHold(family, name string) (name_manager.ReleaseFunc, error) {
	if err := mbk.TryAcquire(family, name); err != nil {
		return nil, err
	}

	stopKeepAlive := make(chan struct{})
	keepAliveDone := make(chan struct{})
	if mbk.options.autoReleaseAfter > 0 {
		go func() {
			for {
				select {
				case <-stopKeepAlive:
					keepAliveDone <- struct{}{}
					break
				case <-mbk.clock.After(mbk.options.autoReleaseAfter / 3):
				}

				if err := mbk.KeepAlive(family, name); err != nil {
					fmt.Fprintf(os.Stderr, "cannot keep alive %s:%s: %v\n", family, name, err)
					break
				}
			}
		}()
	}

	releaseFunc := func() error {
		if mbk.options.autoReleaseAfter > 0 {
			stopKeepAlive <- struct{}{}
			<-keepAliveDone
		}
		if err := mbk.Release(family, name); err != nil {
			return err
		}
		return nil
	}

	return releaseFunc, nil
}

func (mbk *mongoBackend) TryAcquire(family, name string) error {
	ctx := context.Background()

	client, err := mbk.client()
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)

	db := client.Database(mbk.options.database)

	// Let's try to get a lease on this name.
	now := mbk.clock.Now()
	document := bson.M{
		"_id": mbk.leaseId(family, name),
		// partition is used by CosmosDB.
		"partition":         "partition",
		"createdAt":         now,
		"lastHeartBeatDate": now,
		"family":            family,
	}
	_, err = mbk.collection(db, leasedNamesCollection).
		InsertOne(ctx, document)
	if err != nil {
		return name_manager.ErrInUse
	}

	// Now, let's see if the name actually exists.
	names, _ := mbk.List()
	fmt.Printf("NAMES %v\n", names)
	res := mbk.collection(db, dataCollection).
		FindOne(ctx, bson.D{{"family", family}, {"name", name}})
	if res.Err() != nil {
		// The name does not exist.  Release the lease immediately.
		_, err = mbk.collection(db, leasedNamesCollection).
			DeleteOne(ctx, bson.D{{"_id", mbk.leaseId(family, name)}})
		if err != nil {
			return err
		}

		if res.Err() == mongo.ErrNoDocuments {
			return name_manager.ErrNotExist
		}
		return res.Err()
	}

	// The lease was successfully acquired, and the name exists.
	return nil
}

func (mbk *mongoBackend) List() ([]name_manager.Name, error) {
	ctx := context.Background()

	client, err := mbk.client()
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(ctx)

	db := client.Database(mbk.options.database)

	result, err := mbk.collection(db, dataCollection).
		Find(context.Background(), bson.D{})
	if err != nil {
		return nil, err
	}

	var names []name_manager.Name
	for result.Next(ctx) {
		if result.Err() != nil {
			return nil, result.Err()
		}

		name := result.Current.Lookup("name").StringValue()
		family := result.Current.Lookup("family").StringValue()

		leaseResult := mbk.collection(db, leasedNamesCollection).FindOne(
			context.Background(),
			bson.M{
				"_id":       mbk.leaseId(family, name),
				"partition": lockDocumentPartition,
			})
		var updatedAt time.Time
		free := true
		if err := leaseResult.Err(); err != nil {
			if err != mongo.ErrNoDocuments {
				return nil, err
			}
		} else {
			leaseDocument, err := leaseResult.DecodeBytes()
			if err != nil {
				return nil, err
			}
			updatedAt = leaseDocument.Lookup("createdAt").Time().UTC()
			free = false
		}

		names = append(names, name_manager.Name{
			Name:      name,
			Family:    family,
			CreatedAt: result.Current.Lookup("createdAt").Time().UTC(),
			UpdatedAt: updatedAt,
			Free:      free,
		})
	}

	return names, nil
}

func (mbk *mongoBackend) Reset() error {
	ctx := context.Background()

	client, err := mbk.client()
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)

	db := client.Database(mbk.options.database)

	collections := []string{dataCollection, leasedNamesCollection, countersCollection}
	for _, collection := range collections {
		if err := mbk.collection(db, collection).Drop(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (mbk *mongoBackend) client() (*mongo.Client, error) {
	mongoConnectCtx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	return mongo.Connect(mongoConnectCtx, mongo_options.Client().ApplyURI(mbk.options.uri))
}

func (mbk *mongoBackend) collection(db *mongo.Database, name string) *mongo.Collection {
	return db.Collection(mbk.options.collectionPrefix + name)
}

func (mbk *mongoBackend) leaseId(family, name string) string {
	return "_" + mbk.options.collectionPrefix + "lock_" + family + name
}

func (mbk *mongoBackend) releaseZombies(ctx context.Context, db *mongo.Database, family string) error {
	if mbk.options.autoReleaseAfter == 0 {
		return nil
	}
	deadline := mbk.clock.Now().Add(-mbk.options.autoReleaseAfter)
	deleteResult, err := mbk.collection(db, leasedNamesCollection).
		DeleteMany(ctx, bson.M{
			"lastHeartBeatDate": bson.M{"$lt": deadline},
			"family":            family,
		})
	if err != nil {
		return err
	}
	if n := deleteResult.DeletedCount; n > 0 {
		fmt.Fprintf(os.Stderr, "Released %d zombies\n", n)
	}
	return nil
}
