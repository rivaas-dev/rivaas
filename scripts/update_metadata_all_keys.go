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

// list with hash - new actor id
var updateList = map[string]string{
	"": "",
}

func main() {
	client := tyk.NewAPIClient(&tyk.Configuration{
		Host:          host,
		Scheme:        scheme,
		DefaultHeader: map[string]string{"x-tyk-authorization": secret},
	})
	//allKeys, response, err := client.KeysApi.ListKeys(context.Background())
	//fatalAway(err, response, "")
	for keyHash, newActorID := range updateList {
		keyObj, keyResp, err := client.KeysApi.GetKey(context.Background(), keyHash, &tyk.GetKeyOpts{Hashed: optional.NewBool(
			true)})
		fatalAway(err, keyResp, keyHash)

		keyObj.MetaData["actor_id"] = newActorID
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
