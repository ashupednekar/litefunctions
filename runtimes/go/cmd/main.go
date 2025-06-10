package main

import (
	"log"

	"github.com/ashupednekar/litefunctions/runtimes/go/pkg"
)


func main(){
	pkg.LoadSettings()
	err := pkg.StartFunction()	
	if err != nil{
		log.Printf("error starting function: %v", err)
	}
}
