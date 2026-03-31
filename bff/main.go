package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	redis "github.com/redis/go-redis/v9"

	amqp "github.com/rabbitmq/amqp091-go"
)

type (
	Cert struct {
		keyBytes  []byte
		certBytes []byte
		caBytes   []byte
		TLSConfig *tls.Config
	}

	CustomContext struct {
		echo.Context
	}

	ParamID struct {
		ID string `param:"id"`
	}

	Telemetry struct {
		TrainID    int     `json:"train_id"`
		Timestamp  int64   `json:"timestamp"`
		Speed      float64 `json:"speed"`
		Position   float64 `json:"position"`
		Status     string  `json:"status"`
		CertSerial string  `json:"cert_serial"`
	}

	ParamMQ struct {
		ID           string    `param:"id"`
		Command      string    `json:"command"`
		ParamsBool   []bool    `json:"params_bool"`
		ParamsInt    []int     `json:"params_int"`
		ParamsFloat  []float64 `json:"params_float"`
		ParamsString []string  `json:"params_string"`
	}
)

const (
	apiPort    = ":8080"
	serverName = "bff"

	pkiLocalEndpoint   = "http://localhost:8080"
	pkiStagingEndpoint = "http://pki-staging:8080"
	pkiProdEndpoint    = "http://pki-prod:8080"

	redisServer      = "redis-server"
	redisPort        = "6379"
	redisTrainPrefix = "train:"

	rabbitMQStagingEndpoint = "rabbit-server-staging:5671"
	rabbitMQProdEndpoint    = "rabbit-server-prod:5671"
	exchangeName            = "train.commands"
	exchangeType            = "topic"
)

var (
	Version     = "development" // set at build time with -ldflags "-X main.Version=1.0.0"
	Environment = os.Getenv("ENVIRONMENT")

	rateLimitCfg = middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate: 10, Burst: 30, ExpiresIn: 3 * time.Minute,
			},
		),
		DenyHandler: func(context echo.Context, identifier string, err error) error {
			return context.JSON(http.StatusTooManyRequests, map[string]string{"message": "rate limit (10/s, b30) exceeded"})
		},
	}

	redisEndpoint string
	rClient       *redis.Client
	redisPW       string

	rabbitEndpoint string

	issueToken  = os.Getenv("PKIISSUETOKEN")
	pkiEndpoint string
	sanName     string
	cert        *Cert
)

func main() {
	// assign variables based on environment
	switch Environment {
	case "STAGING":
		pkiEndpoint = pkiStagingEndpoint
		sanName = fmt.Sprintf("%s-staging", serverName)
		redisEndpoint = fmt.Sprintf("%s-staging:%s", redisServer, redisPort)
		rabbitEndpoint = rabbitMQStagingEndpoint
	case "PROD":
		pkiEndpoint = pkiProdEndpoint
		sanName = fmt.Sprintf("%s-staging", serverName)
		redisEndpoint = fmt.Sprintf("%s-prod:%s", redisServer, redisPort)
		rabbitEndpoint = rabbitMQProdEndpoint
	default:
		log.Fatalf("'%s' environment, exporter will not have a valid cert.\n", Environment)
	}

	// wait on PKI, then fetch certs
	cert = new(Cert)
	for {
		if err := cert.RegisterWithPKI(); err != nil {
			log.Printf("failed to register with PKI server: %s", err.Error())
		} else {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// read redis pw from env var and unset
	if _, ok := os.LookupEnv("REDIS_USER"); !ok {
		log.Fatalf("'REDIS_USER' environment variable is not set")
	}
	if rpw, ok := os.LookupEnv("REDIS_PASS"); !ok {
		log.Fatalf("'REDIS_PASS' environment variable is not set")
	} else {
		redisPW = rpw
		os.Unsetenv("REDIS_PASS") // unset after reading
	}
	// set up redis client
	rClient = redis.NewClient(&redis.Options{
		Addr:     redisEndpoint,
		Username: os.Getenv("REDIS_USER"),
		Password: redisPW,
	})

	router := makeRouter()
	router.Logger.Fatal(router.Start(apiPort))
}

// makeRouter will set up our API server and URI routes
func makeRouter() *echo.Echo {
	rtr := echo.New()
	// handle CORS issue
	rtr.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{
			"http://localhost:8112",
			"http://127.0.0.1:8112",
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
		AllowCredentials: false,
	}))

	rtr.Use(middleware.Recover())
	rtr.Use(middleware.RateLimiterWithConfig(rateLimitCfg))
	echo.NotFoundHandler = func(ctx echo.Context) error {
		return ctx.JSON(http.StatusNotFound, map[string]string{"message": fmt.Sprintf("not found: URI '%v' - Method '%v'", ctx.Request().RequestURI, ctx.Request().Method)})
	}

	// set up routes
	{
		g := rtr.Group("/spog")

		g.GET("/trains", getTrainsFromRedis)
		g.GET("/train/:id", getTrainDetail)
		g.POST("/train/command/:id", sendRabbitMsg)
	}

	return rtr
}

func getTrainsFromRedis(ctx echo.Context) error {
	var trains []string
	var cursor uint64

	for {
		var ks []string
		ks, cursor = rClient.Scan(ctx.Request().Context(), cursor, "train:*", 0).Val()
		for _, k := range ks {
			trains = append(trains, strings.TrimPrefix(k, redisTrainPrefix))
		}
		if cursor == 0 {
			break
		}
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{"trains": trains})
}

func getTrainDetail(ctx echo.Context) error {
	var params ParamID
	if err := ctx.Bind(&params); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("error binding params / body: %s", err.Error()))
	}
	if params.ID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id parameter must be present (.../4001)")
	}

	data, err := rClient.HGetAll(ctx.Request().Context(), redisTrainPrefix+params.ID).Result()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("error fetching data from amqp: %s", err.Error()))
	}
	if len(data) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "train not found")
	}

	return ctx.JSON(http.StatusOK, data)
}

func sendRabbitMsg(ctx echo.Context) error {
	var params ParamMQ
	if err := ctx.Bind(&params); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("error binding params / body: %s", err.Error()))
	}
	if params.ID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id parameter must be present (.../4001)")
	}

	// connect to RabbitMQ
	mqURL := "amqps://"
	endpointUser := os.Getenv("RABBITMQ_TRAIN_USER")
	endpointPass := os.Getenv("RABBITMQ_TRAIN_PASS")
	if endpointUser != "" && endpointPass != "" {
		mqURL += fmt.Sprintf("%s:%s@", endpointUser, endpointPass)
	}
	mqURL += rabbitEndpoint

	rCfg := amqp.Config{
		TLSClientConfig: cert.TLSConfig,
		// uncomment to override plan user:pw with cert name for external auth
		// SASL: []amqp.Authentication{&amqp.ExternalAuth{}},
	}
	rConn, err := amqp.DialConfig(mqURL, rCfg)
	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, fmt.Errorf("error dialing amqp: %s", err.Error()))
	}
	defer rConn.Close()

	rabbitChannel, err := rConn.Channel()
	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, fmt.Errorf("error creating amqp chan: %s", err.Error()))
	}

	// publish to RabbitMQ
	payload, err := json.MarshalIndent(params, "", " ")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("error marshaling amqp payload: %s", err.Error()))
	}
	err = rabbitChannel.Publish(
		exchangeName,               // Exchange
		"train."+params.ID+".cmds", // Routing Key
		true, false,                // mandatory, immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        payload,
		},
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("error publishing to amqp channel: %s", err.Error()))
	}

	return ctx.NoContent(http.StatusAccepted)
}

func (c *Cert) RegisterWithPKI() error {
	// check if the issue token is set
	if issueToken == "" {
		return fmt.Errorf("ISSUETOKEN environment variable is not set")
	}
	// generate private key and CSR
	var csrBytes []byte
	if pk, err := rsa.GenerateKey(rand.Reader, 4096); err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	} else {
		template := &x509.CertificateRequest{
			Subject: pkix.Name{
				CommonName: serverName,
			},
			DNSNames: []string{sanName},
		}
		if cb, err := x509.CreateCertificateRequest(rand.Reader, template, pk); err != nil {
			return fmt.Errorf("failed to create CSR: %v", err)
		} else {
			csrBytes = cb
			c.keyBytes = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)})
		}
	}

	// call the PKI API to request a certificate for this service
	req, err := http.NewRequest(http.MethodPost, pkiEndpoint+"/issue", bytes.NewReader(csrBytes))
	if err != nil {
		return fmt.Errorf("failed to create CSR HTTP request: %v", err)
	}
	req.Header.Set("X-Auth-Token", issueToken)
	req.Header.Set("X-Cert-Type", "client")
	req.Header.Set("Content-Type", "application/application/octet-stream")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send CSR HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respString := ""
		if b, err := io.ReadAll(resp.Body); err == nil {
			respString = string(b)
		}
		return fmt.Errorf("PKI server returned non-OK status: %s\nResponse: %s", resp.Status, respString)
	}

	if c.certBytes, err = io.ReadAll(resp.Body); err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	// fetch the CA
	resp, err = http.Get(pkiEndpoint + "/ca")
	if err != nil {
		return fmt.Errorf("failed to fetch CA certificate: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respString := ""
		if b, err := io.ReadAll(resp.Body); err == nil {
			respString = string(b)
		}
		return fmt.Errorf("PKI server returned non-OK status when fetching CA cert: %s\nResponse: %s", resp.Status, respString)
	}
	if c.caBytes, err = io.ReadAll(resp.Body); err != nil {
		return fmt.Errorf("failed to read CA certificate from response: %v", err)
	}

	// create keystore
	keystore, err := tls.X509KeyPair(c.certBytes, c.keyBytes)
	if err != nil {
		return fmt.Errorf("failed to combine key and cert: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(c.caBytes)
	c.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{keystore},
		RootCAs:      caCertPool,
	}

	return nil
}
