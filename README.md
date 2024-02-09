# Wattbrews: Central System
**EVSYS: electric vehicle charging central system**

EVSYS implements OCPP 1.6J protocol realisation, to work with modern charging points. It acts as a central system, in terminology of the protocol, which can manage charging points, users, and charging sessions. It is a part of the Wattbrews project, which is a platform for electric vehicle charging infrastructure.

EVSYS includes the following features:
- User management
- Charging point management
- Charging session management
- Events and notifications

#### Other parts of the Wattbrews project:
- [EVSYS-back: backend to provide features for end-user applications](https://github.com/ruslan-hut/evsys-back)
- [EVSYS-front: web application](https://github.com/ruslan-hut/evsys-front)
- [Electrum: payment system integration](https://github.com/ruslan-hut/electrum)
- Wattbrews: mobile application for Android (private repository)
- Nomadus: realisation of OCPI protocol for roaming operations (private repository)

### Quick Installation
EVSYS could be run in a standalone mode without database and other parts. First prepare configuration file `config.yml`. Here is an example of the configuration which will accept all unknown tags and charging points, websocket connections will be established on port 5000, and API will be available on port 5001. 
```yaml
---

is_debug: false
time_zone: UTC
accept_unknown_tag: true
accept_unknown_chp: true
listen:
  type: port
  bind_ip: 0.0.0.0
  port: 5000
  tls_enabled: false
  cert_file: 
  key_file: 
api:
  bind_ip: 0.0.0.0
  port: 5001
  tls_enabled: false
  cert_file: 
  key_file: 
mongo:
  enabled: false
  host: 127.0.0.1
  port: 27017
  user: admin
  password: pass
  database: db
payment:
  enabled: false
  api_url: 127.0.0.1:5002
  api_key: 
telegram:
  enabled: false
  telegram_api_key: 
```
In your charging point settings you should enable OCPP 1.6J protocol and specify the address of the server. According to the configuration file, the address should be `ws://<server_ip>:5000/ws`. 

Then you can build the project and run it.
Build the project with `go build` command:
```bash
go build -o evsys
```
Place the configuration file to `/etc/conf/config.yml`. Then run the following command to start the server:
```bash
evsys -conf=/etc/conf/config.yml
```
After that, the server will be running and waiting for connections from charging points.
To use API, make POST to the endpoint `http://<server_ip>:5001/api`. The body of the request should be a JSON object with the following structure:
```json
{
  "charge_point_id": "Wallbox3",
  "connector_id": 0,
  "feature_name": "GetConfiguration",
  "payload": "AllowOfflineTxForUnknownId"
}
```
In this structure you have to specify the charging point id, connector id if needed, feature name - standard OCPP protocol command name, and payload if the command requires. If a charge point is available, system will send a request to it, wait for answer and return it to the client. If the charge point is not available, system will return an error message. Here is an example of the response:
```json
{
  "configurationKey": [
    {
      "key": "AllowOfflineTxForUnknownId",
      "readonly": false,
      "value": "true"
    }
  ],
  "unknownKey": []
}
``` 
To see connected points id'd you can use the following command, which is not listed in OCPP standard, but implemented in EVSYS:
```json
{
  "charge_point_id": "",
  "connector_id": 0,
  "feature_name": "GetServerStatus",
  "payload": ""
}
```
It will return the list of connected charging points:
```json
{
  "connected_clients":"Wallbox3",
  "total_clients":1
}
```

### Notifications to Telegram
EVSYS could send notifications to Telegram bot. To enable this feature, you have to register bot with Telegram's Bot Father, then specify your bot API key in the configuration file. When enabled, user have to subscribe on notifications by sending a command `/start` to the bot. After that, user will receive notifications about charging sessions, errors, and other events.