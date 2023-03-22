#!/bin/sh

# Create a reminder that will be executed in 55s
curl --location 'localhost:3000/reminder' \
--header 'Content-Type: application/json' \
--data '{
    "actorType": "myactor",
    "actorID": "myid",
    "name": "reminder55",
    "executionTime": "+55s"
}'

# Create a reminder that will be executed in 10s
curl --location 'localhost:3000/reminder' \
--header 'Content-Type: application/json' \
--data '{
    "actorType": "myactor",
    "actorID": "myid",
    "name": "reminder10",
    "executionTime": "+10s"
}'

# Create a reminder that will be executed in 20s
curl --location 'localhost:3000/reminder' \
--header 'Content-Type: application/json' \
--data '{
    "actorType": "myactor",
    "actorID": "myid",
    "name": "reminder20",
    "executionTime": "+20s"
}'

# Sleep for 8s, which means that reminder10 will have been loaded by at least one instance
sleep 8

# Schedule a new reminder that will need to be executed before reminder10
curl --location 'localhost:3000/reminder' \
--header 'Content-Type: application/json' \
--data '{
    "actorType": "actor1",
    "actorID": "myid",
    "name": "reminder1",
    "executionTime": "+1s"
}'