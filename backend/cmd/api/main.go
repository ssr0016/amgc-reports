package main

// @title           Pastoral Reports API
// @version         1.0
// @description     Pastoral Reports backend API: JWT cookie auth and RBAC

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @BasePath  /api/v1

// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name access_token

import (
	"os"

	"github.com/ssr0016/pastoral-reports/backend/cmd/api/app"
)

func main() {
	os.Exit(app.Execute(os.Args[1:], os.Stderr))
}
