package main

import (
	"testproject/mypackage"

	"github.com/sergey-shpilevskiy/wails/v2"
)

func main() {
	// Create application with options
	app := wails.CreateApp("jsbundle", 1024, 768)

	/***** Struct Literal *****/

	// Local struct pointer literal *WORKING*
	app.Bind(&Basic{})

	// External struct pointer literal
	app.Bind(&mypackage.Manager{})

}
