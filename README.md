# time_server
First attempt to write some usefull backend example application in GO

Example of implementation of REST API written in GO. API serving date-time and allowing timezone conversion.

Server side usage:

Default buildin safe settings are:
- Log file "tserver.log"
- Log file size kilobyte "k"
- Log file size 100x unit above
- Log files number 10
- API bound to interface "127.0.0.1"
- API port 8888

Repository contains configuration file config.yaml with the following settings:
- Log file "time_server.log"
- Log file size megabyte "M"
- Log file size 2x unit above
- Log files number 5
- API bound to interface "0.0.0.0
- API port 8080





API documentation:
When server is running API documentation is available at http://Server_Address:Port/