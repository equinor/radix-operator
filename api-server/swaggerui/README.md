# Swagger UI

The Swagger UI comes from [https://github.com/swagger-api/swagger-ui](https://github.com/swagger-api/swagger-ui).

Update Swagger UI version:

- Find latest version and update the version tag in the imports
- Fix the integrity hashes

Ref. [Swagger Installation](https://swagger.io/docs/open-source-tools/swagger-ui/usage/installation/)

## Integrity Hash

Calculate integrity hash when upgrading:

```shell
curl https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.29.1/swagger-ui.js | sha512sum --quiet | xxd -r -p | base64
```
