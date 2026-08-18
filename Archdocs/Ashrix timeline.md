* Rotating of Certificate, Revocation, Renewal etc... 
* Implement of Cert IP, DNS and Common Names (spfie) etc....
* Implement Versioning of Gateway and Connector binary
* Implement Telemetry for Gateway And Connector to CP DB


* Update Policy Bundle Action 

 
* Implement the Cmd Dispatcher 
* Implement each Cmd Action
* Add Timestamp Signature to avoid replay 
* Implement User Revocation Sessions in Gateway and CP

* App Mapping to IDP on CP

* Verify if to add a session hash of CP(in DB) to the gateway identity stored on cache
* To add dispatcher to creation of Connectors for connector sync status
* To add apps and url permitted to be accessible from the gateway to avoid lateral movements, so a dispatcher for apps to sync it with gateway

* Close Quic connection on revocation of connector cert






Testing


* Testing Authorisation



# Connector
* Remove tray from connector and restructure to cli
* Redo connector installer


# AccessLogs and Audit Logging
Accesslog Api to get list with filters
Restructuring of Logging UI
Access logging and audit logging 
Audit log list api


# Flexibility in the Port designing for each service
Now redesign the ports and host for gRPC server on cp, and gRPC/QUIc on gateway 
Intégration of websocket as fallback for connection 


Update the document of the platform with changes.


Intégration testing in general
Frontend Building and integration 


Now learning and verifying the possibility of ML integration in the path of the gateway request 


Create tools for LLM generations of policies.




Deadline: October 2026