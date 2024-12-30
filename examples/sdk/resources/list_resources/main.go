package main

import (
	"fmt"
	"log"

	"github.com/aliyun/aliyun-odps-go-sdk/odps"
	"github.com/aliyun/aliyun-odps-go-sdk/odps/account"
)

func main() {
	// Specify the ini file path
	configPath := "./config.ini"
	conf, err := odps.NewConfigFromIni(configPath)
	if err != nil {
		log.Fatalf("%+v", err)
	}

	aliAccount := account.NewAliyunAccount(conf.AccessId, conf.AccessKey)
	odpsIns := odps.NewOdps(aliAccount, conf.Endpoint)
	// Set the Default Maxcompute project used By Odps instance
	odpsIns.SetDefaultProjectName(conf.ProjectName)
	odpsIns.SetCurrentSchemaName("default")
	resources := odps.NewResources(odpsIns)

	resources.List(func(resource *odps.Resource, err error) {
		if err != nil {
			log.Fatalf("%+v", err)
		}
		fmt.Printf("%+v\n", resource)
	})

	resource1 := resources.Get("bank_customer.txt")
	fmt.Println(resource1.Exist())
}
