package api

import (
	"net/http"
	"time"

	"../convert"
	"../tangle"
	"github.com/gin-gonic/gin"
)

func init() {
	addAPICall("getTips", getTips)
	addAPICall("getTransactionsToApprove", getTransactionsToApprove)
}

func getTips(request Request, c *gin.Context, t time.Time) {
	var tips = []string{}
	for _, tip := range tangle.Tips {
		//if i >= 25 { break }
		tips = append(tips, convert.BytesToTrytes(tip.Hash)[:81])
	}
	c.JSON(http.StatusOK, gin.H{
		"hashes":   tips,
		"duration": getDuration(t),
	})
}

func getTransactionsToApprove(request Request, c *gin.Context, t time.Time) {
	if (request.Depth < tangle.MinTipselDepth) || (request.Depth > tangle.MaxTipselDepth) {
		ReplyError("Invalid depth input", c)
		return
	}

	var reference []byte
	if len(request.Reference) > 0 && !convert.IsTrytes(request.Reference, 81) {
		ReplyError("Wrong reference trytes", c)
		return
	} else if len(request.Reference) > 0 {
		reference = convert.TrytesToBytes(request.Reference)[:49]
	}

	if len(reference) < 49 {
		reference = nil
	}

	tips := tangle.GetTXToApprove(reference, request.Depth)
	if tips == nil {
		ReplyError("Could not get transactions to approve", c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trunkTransaction":  convert.BytesToTrytes(tips[0])[:81],
		"branchTransaction": convert.BytesToTrytes(tips[1])[:81],
		"duration":          getDuration(t),
	})
}