## Mongo Adapter - Faza.io
###### Author: Mostafa Talebi <mostafa@faza.io>
It is an adapter for MongoDB's official golang driver. It provides 
a set of methods for general CRUD operations on MongoDB. 
It is shipped with a test file as well.

Rename `.env.sample` file to`.env`.

To run the tests, simple run the following command:
```bash
make test-full
```
The above command downloads the packages, runs a mongo docker container,
runs the test
and destroys the docker container.

To run it in simple mode, without any self-destructing mongo docker container,
run the following:
```bash
make test-simple
```
Simple mode does not run any mongo docker container.
If you use simple mode, make sure to set proper values in the project's
`.env` file.

If you have all your packages and need only to run test, simply run:
```bash
make test-code
```

#### Methods

```go
FindOne()
FindMany()
FindWhereIn()
InsertOne()
InsertMany()
```