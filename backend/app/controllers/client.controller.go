package controllers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type clientController struct{}

func (e clientController) Report(c *gin.Context) {
	// we need to parse the request
	// var request []*clientmodels.CollectionFrame
	// if err := c.ShouldBindBodyWithJSON(&request); err != nil {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	// 	return
	// }

	b := []byte{}

	c.ShouldBindBodyWithPlain(&b)

	fmt.Println(string(b))

	// tx := middleware.GetTx(c)
	// for _, frame := range request {
	// frame.Metrics
	// frame.StackTraces
	// frame.Transactions
	// }
	// exceptions, err := repositories.ExceptionRepository.FindAll(tx)

	// if err != nil {
	// 	c.AbortWithStatus(http.StatusInternalServerError)
	// 	panic(err)
	// }

	c.JSON(http.StatusOK, gin.H{})
}

var ClientController = clientController{}
