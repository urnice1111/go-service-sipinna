To run the docker container:

cd [rootfoler]
docker run --rm --name sipinna-api --env-file .env -p 8080:8080 --network NetworkSipinna docker-sipinna-go

