
curl --location 'localhost:3000/reminder' \
--header 'Content-Type: application/json' \
--data '{
    "actorType": "myactor",
    "actorID": "myid",
    "name": "reminder55",
    "executionTime": "+55s"
}'

curl --location 'localhost:3000/reminder' \
--header 'Content-Type: application/json' \
--data '{
    "actorType": "myactor",
    "actorID": "myid",
    "name": "reminder10",
    "executionTime": "+10s"
}'


curl --location 'localhost:3000/reminder' \
--header 'Content-Type: application/json' \
--data '{
    "actorType": "myactor",
    "actorID": "myid",
    "name": "reminder20",
    "executionTime": "+20s"
}'

sleep 8

curl --location 'localhost:3000/reminder' \
--header 'Content-Type: application/json' \
--data '{
    "actorType": "actor1",
    "actorID": "myid",
    "name": "reminder1",
    "executionTime": "+1s"
}'