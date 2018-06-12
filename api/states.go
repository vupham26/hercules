package api

import (
	"net/http"
	"time"
	"github.com/gin-gonic/gin"
	"github.com/dgraph-io/badger"
	"gitlab.com/semkodev/hercules/convert"
	"gitlab.com/semkodev/hercules/db"
)

func getInclusionStates (request Request, c *gin.Context, t time.Time) {
	var states = []bool{}
	_ = db.DB.View(func(txn *badger.Txn) error {
		for _, hash := range request.Transactions {
			if !convert.IsTrytes(hash, 81) {
				ReplyError("Wrong hash trytes", c)
				return nil
			}
			states = append(states, db.Has(db.GetByteKey(convert.TrytesToBytes(hash)[:49], db.KEY_CONFIRMED), txn))
		}
		return nil
	})
	c.JSON(http.StatusOK, gin.H{
		"states": states,
		"duration": getDuration(t),
	})
}

func wereAddressesSpentFrom (request Request, c *gin.Context, t time.Time) {
	var states = []bool{}
	_ = db.DB.View(func(txn *badger.Txn) error {
		for _, hash := range request.Addresses {
			if !convert.IsTrytes(hash, 81) {
				ReplyError("Wrong hash trytes", c)
				return nil
			}
			states = append(states, db.Has(db.GetByteKey(convert.TrytesToBytes(hash)[:49], db.KEY_SPENT), txn))
		}
		return nil
	})
	c.JSON(http.StatusOK, gin.H{
		"states": states,
		"duration": getDuration(t),
	})
}