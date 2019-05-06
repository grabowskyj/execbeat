# execbeat
Execbeat for Elasticsearch

I modified a bit the Execbeat for my purposes, thus it also able to:
  - measure the duration of an executed command
  - timeout can be set for a command in the .yml file with key timeout

I just commited the modified files which should be placed to the:
  - execbeat/beater directory in case of exec* files
  - execbeat/config directory in case of config file if you want to use the functionalities.

# How to build
 - Clone elastic/beats and christiangalsterer/execbeat into src/github.com directory
 - Modify GOPATH environment variable so that it contains the parent directory of src/ directory
 - Overwrite the required files
 - go build
