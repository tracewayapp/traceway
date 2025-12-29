package controllers

type exceptionController struct{}

// func (e exceptionController) FindAll(c *gin.Context) {
// 	tx := middleware.GetTx(c)
// 	exceptions, err := repositories.ExceptionRepository.FindAll(tx)

// 	if err != nil {
// 		c.AbortWithStatus(http.StatusInternalServerError)
// 		panic(err)
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"exceptions": exceptions,
// 	})
// }

var ExceptionController = exceptionController{}
