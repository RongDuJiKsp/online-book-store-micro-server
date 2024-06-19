package main

import (
	"io"
	"net/http"
	"net/url"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	goService := gin.Default()
	goService.Use(cors.Default())
	goService.GET("/proxyFile", proxyForFile)
	goService.Run(":3001")
}
func proxyForFile(context *gin.Context) {
	proxyTargetUrl, _ := url.QueryUnescape(context.Query("url"))
	client := http.Client{}
	request, err := http.NewRequest("GET", proxyTargetUrl, nil)
	if err != nil {
		context.String(500, err.Error())
		return
	}
	request.Header.Add("User-Agent", `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36`)
	res, err := client.Do(request)
	if err != nil {
		context.String(500, err.Error())
		return
	}
	for k, vs := range res.Header {
		for _, v := range vs {
			if k == `Access-Control-Allow-Origin` {
				continue
			}
			context.Writer.Header().Add(k, v)
		}
	}
	fileBody, err := io.ReadAll(res.Body)
	if err != nil {
		context.String(500, err.Error())
		return
	}
	context.Data(200, res.Header.Get("Content-Type"), fileBody)
}
