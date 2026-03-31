package main

import (
	"bytes"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"

	echo "github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const (
	webPort = ":8080"

	bffServer      = "localhost" // will be called from javascript so needs to be localhost
	bffPortStaging = ":8111"     // will be called from javascript so needs to be external
	bffPortProd    = ":8211"
)

var (
	Version     = "development" // set at build time with -ldflags "-X main.Version=1.0.0"
	Environment = os.Getenv("ENVIRONMENT")
	bffEndpoint string
)

// embed directive must be used at a top level variable
//
//go:embed index.html
var embeddedHTML embed.FS

func main() {
	// assign variables based on environment
	switch Environment {
	case "STAGING":
		bffEndpoint = fmt.Sprintf("http://%s%s/spog", bffServer, bffPortStaging)
	case "PROD":
		bffEndpoint = fmt.Sprintf("http://%s%s/spog", bffServer, bffPortProd)
	default:
		log.Fatalf("'%s' environment, service will not have a valid API connection.\n", Environment)
	}

	// embed index.html so we can set the BFF API URL
	htmlBytes, err := embeddedHTML.ReadFile("index.html")
	if err != nil {
		log.Fatalf("Failed to read embedded HTML: %s\n", err.Error())
	}

	// replace the BFF API URL
	modHTML := bytes.ReplaceAll(htmlBytes,
		[]byte(`const API_BASE = ''; `),
		[]byte(`const API_BASE = '`+bffEndpoint+`'; `),
	)

	router := makeRouter(modHTML)
	router.Logger.Fatal(router.Start(webPort))
}

// makeRouter will set up our URI routes
func makeRouter(htmlBytes []byte) *echo.Echo {
	rtr := echo.New()

	// fix CORS just for the intended API backend
	rtr.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{
			"*",
		},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
		},
		ExposeHeaders: []string{
			echo.HeaderContentType,
		},
		AllowCredentials: false, // set true only when we need auth or cookies
	}))

	// other pre-flight stuff
	rtr.Use(middleware.Recover())

	// serve the index.html file we've put into modHTML
	rtr.GET("/", func(ctx echo.Context) error {
		return ctx.HTMLBlob(http.StatusOK, htmlBytes)
	})

	// simple health check
	rtr.GET("/health", func(ctx echo.Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"version": Version})
	})

	return rtr
}
