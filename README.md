# Admin API
This project provides APIs that allow for administrative tasks. Please refer to the swagger file for more details about the endpoint.

## How to setup development environment?
1. Start the required containers by running `docker compose up` in the project root.
2. Run `mage run`
3. Access the terminal on container `admin-tools` and execute `tctl --namespace default admin cluster asa --name ReferenceID --type Keyword`
4. The API is reachable via `http://127.0.0.1:8090`
5. The test API Gateway endpoint is reachable via `http://127.0.0.1:8081/test-api`
6. The temporal UI is reachable via http://localhost:8181/
7. Execute command `tctl --namespace default admin cluster asa --name ReferenceID --type Keyword` in `temporal/admin-tools` docker container

## How to setup Keycloak?
1. Start the container but don't spin up the application. 
2. Go to http://localhost:8080/admin
3. Credentials are `admin` and `password`
4. Go to `Client` tab then open client with name `admin-cli`
5. Turn `Client authentication`, `Standard flow` and `Service accounts roles` ON. Press `Save`
6. Go to `Service Accounts Roles` tab then press `Assign Role`, choose `admin` and then press `Assign`.
7. Go to `Credentials` tab then copy `Client secret`
8. Paste this secret in the config.yaml file in `keycloak.clientSecret`
9. Now your KeyCloak is ready to be used.

## Create a key to add to TYK
Once everything is up and running you can use a `curl` command to create a key for TYK

```curl
curl --location 'http://localhost:8090/keys' \
--header 'X-Customer-ID: urn:online:user:000:111' \
--header 'Content-Type: application/json' \
--data-raw '{
  "policies": [
    "policy-1"
  ],
  "actor_id": "SF:THISISATEST",
  "description": "Simple test",
  "expires_at": "2023-10-01",
  "quota": 100,
  "contacts": {
    "emails": ["info@local.host"],
    "users": [1, 2, 3]
  },
  "rate_limit": {
    "rate": 1000,
    "per": 1000
  },
  "environment": "sandbox"
}'
```