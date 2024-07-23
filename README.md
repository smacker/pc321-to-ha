# Owon PC321-W to Home Assistant

Convert messages published by PC321-W to the format understandable by Home Assistant.

## Docker compose

```yaml
services:
  pc321-to-ha:
    image: smacker/pc321-to-ha
    restart: unless-stopped
    user: ${PUID}:${PGID}
    command: -topic device/<broker-user>/report -broker tcp://<broker-ip>:1883 -user <broker-user> -password <broker-password>
```

Replace `<broker-user>`, `<broker-ip>` and `<broker-password>` with the actual values of your MQTT message broker that is connected to PC321-W and Home Assistant.
