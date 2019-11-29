// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package local_backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/benbjohnson/clock"
	bolt "github.com/etcd-io/bbolt"
	"github.com/hchauvin/name_manager/pkg/name_manager"
)

var backendDescription = `Local backend.

The local backend does not rely on any external service and uses the local
file system to maintain a list of names and to enforce a global lock.

The backend URLs have the format "local://<path>", where "<path>" is a
path on the local file system.

The implementation of the local backend leverages a Bolt key-value database,
and "path" is where the DB is located.
`

func init() {
	name_manager.RegisterBackend(name_manager.Backend{
		Protocol:          "local",
		Description:       backendDescription,
		CreateNameManager: createNameManager,
	})
}

func createNameManager(backendURL string) (name_manager.NameManager, error) {
	path, options, err := parseBackendURL(backendURL)
	if err != nil {
		return nil, err
	}
	return &localBackend{
		path:    path,
		clock:   clock.New(),
		options: *options,
	}, nil
}

type localBackend struct {
	// path is the local path where the Bolt DB is stored.
	path string
	// clock is the clock used to get the CreatedAt/UpdatedAt timestamps.
	clock clock.Clock
	// options are the options for the backend.
	options options
}

// localBackendData contains the metadata associated to a name.
type localBackendData struct {
	// CreatedAt is the time at which the name was created.
	// It is marshalled to the RFC3339 format.
	CreatedAt time.Time `json:"createdAt"`
	// Update is the time at which the name was updated.  This time is
	// not changed when the name is released, only when it is acquired
	// again or kept alive.  It is marshalled to the RFC3339 format.
	UpdatedAt time.Time `json:"updatedAt"`
}

// familyNameSep is the separator between the family and the name in the DB keys.
const familyNameSep = ":"

var (
	// dataBucket is the name of the Bolt bucket that contains the metadata
	// associated to a name.  In this bucket, there is one entry per name,
	// the keys have the format `<family>:<name>` and the values are
	// json-marshalled `localBackendData` objects.
	dataBucket = []byte("data")

	// freeNames bucket is the name of the Bolt bucket that is used to
	// keep track of all the free names.  In this bucket, there is one
	// entry per free name, the keys have the format `<family>:<name>`
	// and the values are all set to the `freeValue` placeholder.
	freeNamesBucket = []byte("freeNames")

	// countersBucket is the name of the Bolt bucket that is used to
	// keep track of the number of names for each family.  In this
	// bucket, there is one entry per family, the keys are the family
	// names, and the values are itoa-formatted counters.
	countersBucket = []byte("counters")

	// freeValue is the placeholder that is used for the values in
	// `freeNamesBucket`, as this bucket is only used for its keys and
	// its values have no meaning at all.
	freeValue = []byte("free")
)

func (lbk *localBackend) Hold(family string) (string, name_manager.ReleaseFunc, error) {
	name, err := lbk.Acquire(family)
	if err != nil {
		return "", nil, err
	}

	stopKeepAlive := make(chan struct{})
	keepAliveDone := make(chan struct{})
	if lbk.options.autoReleaseAfter > 0 {
		go func() {
			for {
				select {
				case <-stopKeepAlive:
					keepAliveDone <- struct{}{}
					break
				case <-lbk.clock.After(lbk.options.autoReleaseAfter / 3):
				}

				if err := lbk.KeepAlive(family, name); err != nil {
					fmt.Fprintf(os.Stderr, "cannot keep alive %s:%s: %v\n", family, name, err)
					break
				}
			}
		}()
	}

	releaseFunc := func() error {
		if lbk.options.autoReleaseAfter > 0 {
			stopKeepAlive <- struct{}{}
			<-keepAliveDone
		}
		if err := lbk.Release(family, name); err != nil {
			return err
		}
		return nil
	}

	return name, releaseFunc, nil
}

func (lbk *localBackend) Acquire(family string) (string, error) {
	db, err := lbk.openDB()
	if err != nil {
		return "", err
	}
	defer db.Close()

	name := ""
	if err := db.Update(func(tx *bolt.Tx) error {
		if autoReleaseAfter := lbk.options.autoReleaseAfter; autoReleaseAfter > 0 {
			if err := releaseZombies(tx, lbk.clock, autoReleaseAfter, family); err != nil {
				return err
			}
		}
		n, err := acquire(tx, lbk.clock, family)
		if err != nil {
			return err
		}
		name = n
		return nil
	}); err != nil {
		return "", err
	}
	return name, nil
}

func (lbk *localBackend) KeepAlive(family, name string) error {
	db, err := lbk.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Update(func(tx *bolt.Tx) error {
		return keepAlive(tx, lbk.clock, family, name)
	})
}

func (lbk *localBackend) Release(family, name string) error {
	db, err := lbk.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Update(func(tx *bolt.Tx) error {
		return release(tx, family, name)
	})
}

func (lbk *localBackend) TryHold(family, name string) (name_manager.ReleaseFunc, error) {
	if err := lbk.TryAcquire(family, name); err != nil {
		return nil, err
	}

	stopKeepAlive := make(chan struct{})
	keepAliveDone := make(chan struct{})
	if lbk.options.autoReleaseAfter > 0 {
		go func() {
			for {
				select {
				case <-stopKeepAlive:
					keepAliveDone <- struct{}{}
					break
				case <-lbk.clock.After(lbk.options.autoReleaseAfter / 3):
				}

				if err := lbk.KeepAlive(family, name); err != nil {
					fmt.Fprintf(os.Stderr, "cannot keep alive %s:%s: %v\n", family, name, err)
					break
				}
			}
		}()
	}

	releaseFunc := func() error {
		if lbk.options.autoReleaseAfter > 0 {
			stopKeepAlive <- struct{}{}
			<-keepAliveDone
		}
		if err := lbk.Release(family, name); err != nil {
			return err
		}
		return nil
	}

	return releaseFunc, nil
}

func (lbk *localBackend) TryAcquire(family, name string) error {
	db, err := lbk.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Update(func(tx *bolt.Tx) error {
		return tryAcquire(tx, lbk.clock, family, name)
	})
}

func (lbk *localBackend) List() ([]name_manager.Name, error) {
	db, err := lbk.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var lst []name_manager.Name
	if err = db.View(func(tx *bolt.Tx) error {
		l, err := list(tx)
		if err != nil {
			return err
		}
		lst = l
		return nil
	}); err != nil {
		return nil, err
	}
	return lst, nil
}

func (lbk *localBackend) Reset() error {
	err := os.Remove(lbk.path)
	if err != nil {
		if pathError, ok := err.(*os.PathError); ok {
			if pathError.Err.Error() == "no such file or directory" {
				return nil
			}
		}
	}
	return err
}

// openDB opens the Bolt DB associated with this local backend.
func (lbk *localBackend) openDB() (*bolt.DB, error) {
	return bolt.Open(lbk.path, 0666, nil)
}

// acquire implements name acquisition inside a Bolt transaction.
func acquire(tx *bolt.Tx, clk clock.Clock, family string) (string, error) {
	nameBytes, err := getAnyFreeName(tx, family)
	if err != nil {
		return "", err
	}
	var name string
	var data *localBackendData
	now := clk.Now().UTC()
	if nameBytes != nil {
		name = string(nameBytes)
		data, err = getData(tx, family, name)
		if err != nil {
			return "", err
		}
		if data == nil {
			return "", fmt.Errorf("inconsistent database")
		}
		if err = removeFreeName(tx, family, name); err != nil {
			return "", err
		}
	} else {
		counter, err := getAndIncrementCounter(tx, family)
		if err != nil {
			return "", err
		}
		name = strconv.Itoa(counter)
		nameBytes = []byte(name)
		data = &localBackendData{
			CreatedAt: now,
		}
	}
	data.UpdatedAt = now
	if err = setData(tx, family, name, data); err != nil {
		return "", err
	}
	return name, nil
}

// keepAlive implements keep alive inside a Bolt transaction.
func keepAlive(tx *bolt.Tx, clk clock.Clock, family, name string) error {
	if isNameFree(tx, family, name) {
		return nil
	}
	// We only keep alive if the name has some data associated to it.
	data, err := getData(tx, family, name)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	data.UpdatedAt = clk.Now().UTC()
	if err = setData(tx, family, name, data); err != nil {
		return err
	}
	return nil
}

// release implements name release inside a Bolt transaction.
func release(tx *bolt.Tx, family, name string) error {
	if isNameFree(tx, family, name) {
		return nil
	}
	// We only add a free name if the name has some data associated to
	// it.
	dat, err := getData(tx, family, name)
	if err != nil {
		return err
	}
	if dat == nil {
		return nil
	}
	return addFreeName(tx, family, name)
}

// tryAcquire implements name acquisition inside a Bolt transaction.
func tryAcquire(tx *bolt.Tx, clk clock.Clock, family, name string) error {
	if !isNameFree(tx, family, name) {
		return name_manager.ErrInUse
	}

	var data *localBackendData
	now := clk.Now().UTC()
	data, err := getData(tx, family, name)
	if err != nil {
		return err
	}
	if data == nil {
		return name_manager.ErrNotExist
	}
	if err = removeFreeName(tx, family, name); err != nil {
		return err
	}
	data.UpdatedAt = now
	if err = setData(tx, family, name, data); err != nil {
		return err
	}
	return nil
}

// list implements name listing inside a Bolt transaction.
func list(tx *bolt.Tx) ([]name_manager.Name, error) {
	var names []name_manager.Name

	b := tx.Bucket(dataBucket)
	if b == nil {
		return nil, nil
	}
	c := b.Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		family, name := keyToFamilyName(k)
		data := localBackendData{}
		if err := json.Unmarshal(v, &data); err != nil {
			return nil, err
		}
		free := isNameFree(tx, family, name)
		var updatedAt time.Time
		if !free {
			updatedAt = data.UpdatedAt
		}
		names = append(names, name_manager.Name{
			Name:      name,
			Family:    family,
			CreatedAt: data.CreatedAt,
			UpdatedAt: updatedAt,
			Free:      free,
		})
	}

	return names, nil
}

func releaseZombies(tx *bolt.Tx, clk clock.Clock, autoReleaseAfter time.Duration, family string) error {
	b, err := tx.CreateBucketIfNotExists(dataBucket)
	if err != nil {
		return err
	}
	now := clk.Now()
	prefix := []byte(family + familyNameSep)
	c := b.Cursor()
	for k, v := c.Seek(prefix); k != nil; k, v = c.Next() {
		if !bytes.HasPrefix(k, prefix) {
			break
		}
		data := &localBackendData{}
		if err := json.Unmarshal(v, data); err != nil {
			return err
		}
		if now.Sub(data.UpdatedAt) > autoReleaseAfter {
			_, name := keyToFamilyName(k)
			if err := release(tx, family, name); err != nil {
				return err
			}
		}
	}
	return nil
}

// getAnyFreeName returns any free name for the given family, or `nil`
// if there is no such name, in which case a new name must be generated.
func getAnyFreeName(tx *bolt.Tx, family string) ([]byte, error) {
	b, err := tx.CreateBucketIfNotExists(freeNamesBucket)
	if err != nil {
		return nil, err
	}
	prefix := []byte(family + familyNameSep)
	k, _ := b.Cursor().Seek(prefix)
	if k == nil {
		return nil, nil
	}
	if !bytes.HasPrefix(k, prefix) {
		return nil, nil
	}
	return k[len(prefix):], nil
}

// isNameFree returns whether a name is free.
func isNameFree(tx *bolt.Tx, family string, name string) bool {
	b := tx.Bucket(freeNamesBucket)
	if b == nil {
		// If there is no freeNames bucket, it means that getAnyFreeName
		// was never called.  Therefore, the name is considered free.
		return true
	}
	v := b.Get(familyNameToKey(family, name))
	if v == nil {
		return false
	}
	return true
}

// removeFreeName removes a name from the list of free name.  It errors,
// among other things, when the name is not free (see `isNameFree`).
func removeFreeName(tx *bolt.Tx, family string, name string) error {
	b := tx.Bucket(freeNamesBucket)
	if b == nil {
		panic(fmt.Sprintf("expected bucket %s to exist", freeNamesBucket))
	}
	return b.Delete(familyNameToKey(family, name))
}

// addFreeName adds a name to the list of free names for a given family.
func addFreeName(tx *bolt.Tx, family string, name string) error {
	b, err := tx.CreateBucketIfNotExists(freeNamesBucket)
	if err != nil {
		return err
	}
	return b.Put(familyNameToKey(family, name), freeValue)
}

// getAndIncrementCounter gets the number of names registered for a
// family and increments it.  The return value is therefore
// `counters[family]++`.
func getAndIncrementCounter(tx *bolt.Tx, family string) (int, error) {
	b, err := tx.CreateBucketIfNotExists(countersBucket)
	if err != nil {
		return 0, err
	}
	counterKey := []byte(family)
	counterBytes := b.Get(counterKey)
	var counter int
	if counterBytes == nil {
		counter = 0
	} else {
		i, err := strconv.Atoi(string(counterBytes))
		if err != nil {
			return 0, err
		}
		counter = i
	}
	if err = b.Put(counterKey, []byte(strconv.Itoa(counter+1))); err != nil {
		return 0, err
	}
	return counter, nil
}

// getData returns the metadata for a given name, or `nil` if the metadata
// could not be found.
func getData(tx *bolt.Tx, family string, name string) (*localBackendData, error) {
	b := tx.Bucket(dataBucket)
	if b == nil {
		return nil, nil
	}
	jsonData := b.Get(familyNameToKey(family, name))
	if jsonData == nil {
		return nil, nil
	}
	data := &localBackendData{}
	if err := json.Unmarshal(jsonData, data); err != nil {
		return nil, err
	}
	return data, nil
}

// setData creates or updates the metadata for a given name.
func setData(tx *bolt.Tx, family string, name string, data *localBackendData) error {
	dataJson, err := json.Marshal(data)
	if err != nil {
		return err
	}
	b, err := tx.CreateBucketIfNotExists(dataBucket)
	if err != nil {
		return err
	}
	return b.Put(familyNameToKey(family, name), dataJson)
}

// familyNameToKey gets a key from a family and a name.
func familyNameToKey(family string, name string) []byte {
	return []byte(family + familyNameSep + name)
}

// keyToFamilyName parses a key to get a family and a name.
func keyToFamilyName(key []byte) (family string, name string) {
	parts := strings.Split(string(key), ":")
	if len(parts) != 2 {
		panic(fmt.Sprintf("invalid key '%s'", key))
	}
	family = parts[0]
	name = parts[1]
	return
}
