# AWS IoT Core

## Recommended Policies

If you are planning to ues topic namespace `example.com/{ThingName}` for your devices, we recommend following policy manifest for your devices:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "iot:Publish"
      ],
      "Resource": [
        "arn:aws:iot:REGION:ACCOUNT_ID:topic/example.com/${iot:Connection.Thing.ThingName}/msg",
        "arn:aws:iot:REGION:ACCOUNT_ID:topic/example.com/${iot:Connection.Thing.ThingName}/status"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "iot:Subscribe"
      ],
      "Resource": [
        "arn:aws:iot:REGION:ACCOUNT_ID:topicfilter/example.com/${iot:Connection.Thing.ThingName}/cmd"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "iot:Receive"
      ],
      "Resource": [
        "arn:aws:iot:REGION:ACCOUNT_ID:topic/example.com/${iot:Connection.Thing.ThingName}/cmd"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "iot:Connect"
      ],
      "Resource": [
        "arn:aws:iot:REGION:ACCOUNT_ID:client/${iot:Connection.Thing.ThingName}"
      ]
    }
  ]
}
```

and following policy manifest for `EdgeDevice`s:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "iot:Publish"
      ],
      "Resource": [
        "arn:aws:iot:REGION:ACCOUNT_ID:topic/example.com/${iot:Connection.Thing.Attributes[client]}/cmd"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "iot:Subscribe"
      ],
      "Resource": [
        "arn:aws:iot:REGION:ACCOUNT_ID:topicfilter/example.com/${iot:Connection.Thing.Attributes[client]}/msg",
        "arn:aws:iot:REGION:ACCOUNT_ID:topicfilter/example.com/${iot:Connection.Thing.Attributes[client]}/status"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "iot:Receive"
      ],
      "Resource": [
        "arn:aws:iot:REGION:ACCOUNT_ID:topic/example.com/${iot:Connection.Thing.Attributes[client]}/msg",
        "arn:aws:iot:REGION:ACCOUNT_ID:topic/example.com/${iot:Connection.Thing.Attributes[client]}/status"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "iot:Connect"
      ],
      "Resource": [
        "arn:aws:iot:REGION:ACCOUNT_ID:client/${iot:Connection.Thing.ThingName}"
      ]
    }
  ]
}
```

*Replace `REGION` and `ACCOUNT_ID` with your iot core specific values

With the policies above, your edge devices and `EdgeDevice`s must meet the following requirements:

- for edge devices
  - topic namespace: `example.com/{ThingName}`
  - `clientID: {ThingName}`

- for `EdgeDevice`s
  - topic namespace: `example.com/{edge device ThingName}`
  - Thing attribute
    - `client: {edge device ThingName}`
  - `clientID: {ThingName}`
