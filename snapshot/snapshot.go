package snapshot

import (
	"time"

	"../db"
	"../logs"
	"github.com/dgraph-io/badger"
	"github.com/spf13/viper"
)

const (
	SNAPSHOT_SEPARATOR         = "==="
	TIMESTAMP_MIN              = 1525017600
	WAIT_SNAPSHOT_DURATION     = time.Duration(3) * time.Second
	MIN_SPENT_ADDRESSES        = 521970
	MAX_LATEST_TRANSACTION_AGE = 300
)

var TOTAL_IOTAS int64 = 2779530283277761
var keySnapshotDate = []byte{db.KEY_SNAPSHOT_DATE}
var keySnapshotLock = []byte{db.KEY_SNAPSHOT_LOCK}
var keySnapshotFile = []byte{db.KEY_SNAPSHOT_FILE}

var edgeTransactions chan *[]byte
var config *viper.Viper
var CurrentTimestamp = 0
var InProgress = false
var lowEndDevice = false

func Start(cfg *viper.Viper) {
	config = cfg
	logs.Log.Debug("Loading snapshots module")
	edgeTransactions = make(chan *[]byte, 10000000)

	lowEndDevice = config.GetBool("light")
	CurrentTimestamp = GetSnapshotTimestamp(nil)
	logs.Log.Infof("Current snapshot timestamp: %v", CurrentTimestamp)

	// LoadIRISnapshot("snapshotMainnet.txt", "previousEpochsSpentAddresses.txt", 1525017600)
	// LoadAddressBytes("snapshotMainnet.txt")

	go trimTXRunner()

	checkPendingSnapshot()
	go startAutosnapshots()

	snapshotToLoad := config.GetString("snapshots.loadFile")
	iri1 := config.GetString("snapshots.loadIRIFile")
	iri2 := config.GetString("snapshots.loadIRISpentFile")
	iriTimestamp := config.GetInt("snapshots.loadIRITimestamp")
	if len(snapshotToLoad) > 0 {
		LoadSnapshot(snapshotToLoad)
	} else if len(iri1) > 0 && len(iri2) > 0 && iriTimestamp > 0 {
		LoadIRISnapshot(iri1, iri2, iriTimestamp)
	}

	if !checkDatabaseSnapshot() {
		logs.Log.Fatalf("Database is in an inconsistent state. Try deleting it and loading a snapshot.")
	}
}

/*
Sets the current snapshot date in the database
*/
func SetSnapshotTimestamp(timestamp int, txn *badger.Txn) error {
	err := db.Put(keySnapshotDate, timestamp, nil, txn)
	if err == nil {
		CurrentTimestamp = timestamp
	}
	return err
}

/*
Returns timestamp if snapshot lock is present. Otherwise negative number.
If this is a file lock (snapshot being loaded from a file)
*/
func IsLocked(txn *badger.Txn) (timestamp int, filename string) {
	return GetSnapshotLock(txn), GetSnapshotFileLock(txn)
}

/*
Creates a snapshot lock in the database
*/
func Lock(timestamp int, filename string, txn *badger.Txn) error {
	InProgress = true
	err := db.Put(keySnapshotLock, timestamp, nil, txn)
	if err != nil {
		return err
	}
	return db.Put(keySnapshotFile, filename, nil, txn)
}

/*
Removes a snapshot lock in the database
*/
func Unlock(txn *badger.Txn) error {
	InProgress = false
	err := db.Remove(keySnapshotLock, txn)
	if err != nil {
		return err
	}
	return db.Remove(keySnapshotFile, txn)
}

/*
Returns the date unix timestamp of the last snapshot
*/
func GetSnapshotLock(txn *badger.Txn) int {
	timestamp, err := db.GetInt(keySnapshotLock, txn)
	if err != nil {
		return -1
	}
	return timestamp
}

/*
Returns the date unix timestamp of the last snapshot
*/
func GetSnapshotTimestamp(txn *badger.Txn) int {
	if CurrentTimestamp > 0 {
		return CurrentTimestamp
	}

	timestamp, err := db.GetInt(keySnapshotDate, txn)
	if err != nil {
		return -1
	}
	return timestamp
}

/*
Returns the date unix timestamp of the last snapshot
*/
func GetSnapshotFileLock(txn *badger.Txn) string {
	filename, err := db.GetString(keySnapshotFile, txn)
	if err != nil {
		return ""
	}
	return filename
}

/*
Starts a periodic snapshot runner
*/
func startAutosnapshots() {
	snapshotPeriod := config.GetInt("snapshots.period")
	snapshotInterval := config.GetInt("snapshots.interval")
	if snapshotInterval == 0 {
		return
	}
	logs.Log.Infof("Automatic snapshots will be done every %v hours, keeping the past %v hours.",
		snapshotInterval, snapshotPeriod)
	ticker := time.NewTicker(time.Duration(60*snapshotInterval) * time.Minute)
	for range ticker.C {
		logs.Log.Info("Starting automatic snapshot...")
		if !InProgress {
			timestamp := int(time.Now().Unix()) - (snapshotPeriod * 3600)
			MakeSnapshot(timestamp, "")
		} else {
			logs.Log.Warning("D'oh! A snapshot is already in progress. Skipping current run.")
		}
	}

}
