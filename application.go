package fire

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
	"github.com/labstack/echo/middleware"
	"gopkg.in/mgo.v2"
)

// An Application provides an out-of-the-box configuration of components to
// get started with building JSON APIs.
type Application struct {
	disableCompression bool
	disableRecovery    bool

	set    *Set
	router *echo.Echo
}

// New creates and returns a new Application.
func New(mongoURI, prefix string) *Application {
	// create router
	router := echo.New()

	// connect to database
	sess, err := mgo.Dial(mongoURI)
	if err != nil {
		panic(err)
	}

	set := NewSet(sess, router, prefix)

	return &Application{
		set:    set,
		router: router,
	}
}

// EnableCORS will enable CORS with a general configuration.
//
// Note: You can always add your own CORS middleware to the router.
func (a *Application) EnableCORS(origins ...string) {
	a.router.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: origins,
		// TODO: Allow "Accept, Cache-Control"?
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderAuthorization,
			echo.HeaderContentType, echo.HeaderXHTTPMethodOverride},
	}))
}

// EnableBodyLimit will enable request body limitation. You can pass a size in
// the form of 4K, 2M, 1G or 1P. If no value is specified the default value 4K
// will be used instead.
func (a *Application) EnableBodyLimit(size ...string) {
	limit := "4K"
	if len(size) > 0 {
		limit = size[0]
	}

	a.router.Use(middleware.BodyLimit(limit))
}

// EnableMethodOverriding will enable the usage of the X-HTTP-Method-Override
// header to set a request method when using the POST method.
func (a *Application) EnableMethodOverriding() {
	a.router.Pre(middleware.MethodOverride())
}

// Mount will add controllers to the set and register them on the router.
//
// Note: Each controller should only be mounted once.
func (a *Application) Mount(controllers ...*Controller) {
	a.set.Mount(controllers...)
}

// Router will return the internally used echo instance.
func (a *Application) Router() *echo.Echo {
	return a.router
}

// DisableCompression will turn of gzip compression.
//
// Note: This method must be called before calling Run or Start.
func (a *Application) DisableCompression() {
	a.disableCompression = true
}

// DisableRecovery will disable the automatic recover mechanism.
//
// Note: This method must be called before calling Run or Start.
func (a *Application) DisableRecovery() {
	a.disableRecovery = true
}

// Run will run the application using the passed server.
func (a *Application) Run(server engine.Server) {
	// enable gzip compression
	if !a.disableCompression {
		a.router.Use(middleware.Gzip())
	}

	// enable automatic recovery
	if !a.disableRecovery {
		a.router.Use(middleware.Recover())
	}

	a.router.Run(server)
}

// Start will run the application on the specified address.
func (a *Application) Start(addr string) {
	a.Run(standard.New(addr))
}
