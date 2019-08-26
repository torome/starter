package manager

import (
	"github.com/gin-gonic/gin"
	"starter/internal/entities"
	"starter/internal/manager/controllers"
	"starter/pkg/app"
	"starter/pkg/managers"
	"starter/pkg/middlewares"
	"starter/pkg/sessions"
)

var engine = gin.Default()

func GetEngine() *gin.Engine {
	engine.Static("/test", "./test")

	engine.Use(middlewares.CORS)
	engine.Use(middlewares.VerifyAuth)
	sessions.Start(engine)

	engine.POST("/upload", app.Upload("file", "./test/", "http://manager.golang-project.com/", "png", "jpg"))

	engine.Use(middlewares.CsrfToken)

	managers.New().
		Register(entities.Staff{}, managers.Mongo).
		Register(entities.Mgo{}, managers.Mgo).
		RegisterCustomManager(&controllers.CustomOrder{}, entities.Order{}).
		Start(engine)

	managers.New().
		Register(entities.Staff{}, managers.Mongo).
		Register(entities.Mgo{}, managers.Mgo).
		Start(engine.Group("/container"))

	return engine
}
