package local_backend

import (
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
	path, err := expandHome(backendURL)
	if err != nil {
		return nil, err
	}
	return &localBackend{
		path:  path,
		clock: clock.New(),
	}, nil
}

type localBackend struct {
	// path is the local path where the Bolt DB is stored.
	path string
	// clock is the clock used to get the CreatedAt/UpdatedAt timestamps.
	clock clock.Clock
}

type localBackendData struct {
	// CreatedAt is the time at which the name was created.
	// It is marshalled to the RFC3339 format.
	CreatedAt time.Time `json:"createdAt"`
	// Update is the time at which the name was updated.  This time is
	// not changed when the name is released, only when it is acquired
	// again.  It is marshalled to the RFC3339 format.
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

func (lbk *localBackend) Acquire(family string) (string, error) {
	db, err := lbk.openDB()
	if err != nil {
		return "", err
	}
	defer db.Close()

	name := ""
	if err := db.Update(func(tx *bolt.Tx) error {
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

// acquire implements name release inside a Bolt transaction.
func release(tx *bolt.Tx, family, name string) error {
	if isNameFree(tx, family, name) {
		return nil
	}
	return addFreeName(tx, family, name)
}

// acquire implements name listing inside a Bolt transaction.
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
		names = append(names, name_manager.Name{
			Name:      name,
			Family:    family,
			CreatedAt: data.CreatedAt,
			UpdatedAt: data.UpdatedAt,
			Free:      isNameFree(tx, family, name),
		})
	}

	return names, nil
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
	return k[len(family)+1:], nil
}

// isNameFree returns whether a name is free.
func isNameFree(tx *bolt.Tx, family string, name string) bool {
	b := tx.Bucket(freeNamesBucket)
	if b == nil {
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
