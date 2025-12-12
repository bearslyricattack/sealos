module github.com/labring/sealos/service/devboxmonitor

go 1.23.0

require (
	github.com/gin-gonic/gin v1.10.0
	github.com/labring/sealos/service/pkg v0.0.0
)

replace github.com/labring/sealos/service/pkg => ../pkg
