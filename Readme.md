# JWT and Account CRUD

## Endpoints

- /login POST
- /account POST
- /account GET
- /account/{id} GET
- /account/{id} DELETE
- /account/{id} PUT
- /transfer POST

# Set up

## Prerequisites

1. Install docker or postgreSQL locally.
2. If you installed docker then create a postgreSQL container.
	Take a look at https://www.docker.com/blog/how-to-use-the-postgres-docker-official-image/ to learn how to do it.
3. Set the following environment variables:
	```
	export POSTGRES_USERNAME=<your-username>
	export POSTGRES_PASSWORD=<your-password>
	```

## How to start up the server

```
make
./bin/go-bank --seed
```
