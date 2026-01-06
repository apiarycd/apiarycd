// Package main ApiaryCD GitOps platform API
//
//	@title			ApiaryCD API
//	@version		1.0.0
//	@description	ApiaryCD is a GitOps platform for managing Docker Swarm stacks
//	@termsOfService	http://swagger.io/terms/
//
//	@contact.name	API Support
//	@contact.url	https://apiarycd.com/support
//	@contact.email	support@apiarycd.com
//
//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html
//
//	@host			localhost:3000
//	@BasePath		/api/v1
package main

import "github.com/apiarycd/apiarycd/internal"

//go:generate swag init --parseDependency --outputTypes go -g ./main.go -o ./internal/server/docs

func main() {
	internal.Run()
}
