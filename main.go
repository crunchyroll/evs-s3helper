package main

// CONF - the config path location
const CONF = "config/config.yml"

func main() {
	api := App{}
	api.Initialize(CONF)
	api.Run()
}
