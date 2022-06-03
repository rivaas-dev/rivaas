package main

import (
	"context"
	"github.com/antihax/optional"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-sdk-go"
	"log"
	"net/http"
)

const (
	// acc or prod, depending on the secret
	host   = "localhost:9696"
	scheme = "http"
	secret = ""
)

func main() {
	client := tyk.NewAPIClient(&tyk.Configuration{
		Host:          host,
		Scheme:        scheme,
		DefaultHeader: map[string]string{"x-tyk-authorization": secret},
	})
	allKeys, response, err := client.KeysApi.ListKeys(context.Background())
	fatalAway(err, response, "")
	for _, keyHash := range allKeys.Keys {
		keyObj, keyResp, err := client.KeysApi.GetKey(context.Background(), keyHash, &tyk.GetKeyOpts{Hashed: optional.NewBool(
			true)})
		fatalAway(err, keyResp, keyHash)
		_, ok := keyObj.MetaData["actor_id"]
		if ok {
			// means it's already there
			continue
		}
		customerID, ok := keyObj.MetaData["customer_id"]
		if !ok {
			// means there is nothing to copy from
			continue
		}
		keyObj.MetaData["actor_id"] = customerID
		_, keyResp, err = client.KeysApi.UpdateKey(context.Background(), keyHash, &tyk.UpdateKeyOpts{
			Hashed:       optional.NewBool(true),
			SessionState: optional.NewInterface(keyObj),
		})
		fatalAway(err, keyResp, keyHash)
	}
}

func fatalAway(err error, keyResp *http.Response, keyhash string) {
	if keyhash != "" && err != nil {
		log.Fatalf("%s: %s", err.Error(), keyhash)
	}
	if keyResp.StatusCode != http.StatusOK {
		log.Fatalf("bad response from tyk for key %s", keyhash)
	}
	if err != nil {
		log.Fatal(err)
	}

}
