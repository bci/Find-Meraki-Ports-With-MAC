# Sample Run

```
$ $bin\Find-Meraki-Ports-With-MAC-windows-amd64.exe --mac 00:11:22:33:44:55 --org "My Organization" --verbose --log-level info
2026-02-13T12:40:23-08:00 [INFO] MAC: 00:11:22:33:44:55
2026-02-13T12:40:23-08:00 [INFO] Organization: My Organization
2026-02-13T12:40:24-08:00 [INFO] Network: Network1
2026-02-13T12:40:28-08:00 [INFO] Network clients API returned 1000 clients
2026-02-13T12:40:28-08:00 [INFO] Querying switch: switch1 (XXXX-XXXX-XXXX)
2026-02-13T12:41:02-08:00 [INFO] Device clients API returned 0 clients for switch1
2026-02-13T12:41:02-08:00 [INFO] Querying switch: switch2 (XXXX-XXXX-XXXX)
2026-02-13T12:41:05-08:00 [INFO] Live MAC table returned 147 entries for switch2
2026-02-13T12:41:05-08:00 [INFO] Querying switch: switch3 (XXXX-XXXX-XXXX)
2026-02-13T12:41:07-08:00 [INFO] Live MAC table returned 26 entries for switch3
2026-02-13T12:41:07-08:00 [INFO] Querying switch: switch4 (XXXX-XXXX-XXXX)
2026-02-13T12:41:10-08:00 [INFO] Live MAC table returned 1087 entries for switch4
2026-02-13T12:41:10-08:00 [INFO] Querying switch: switch5 (XXXX-XXXX-XXXX)
2026-02-13T12:41:13-08:00 [INFO] Live MAC table returned 1133 entries for switch5
2026-02-13T12:41:13-08:00 [INFO] Querying switch: switch6 (XXXX-XXXX-XXXX)
2026-02-13T12:41:16-08:00 [INFO] Live MAC table returned 2330 entries for switch6
2026-02-13T12:41:16-08:00 [INFO] Network: Network2
2026-02-13T12:41:24-08:00 [INFO] Network clients API returned 1000 clients
2026-02-13T12:41:24-08:00 [INFO] Querying switch: switch7 (XXXX-XXXX-XXXX)
2026-02-13T12:41:58-08:00 [INFO] Device clients API returned 0 clients for switch7
2026-02-13T12:41:58-08:00 [INFO] Querying switch: switch8 (XXXX-XXXX-XXXX)
2026-02-13T12:42:01-08:00 [INFO] Live MAC table returned 71 entries for switch8
2026-02-13T12:42:01-08:00 [INFO] Querying switch: switch9 (XXXX-XXXX-XXXX)
2026-02-13T12:42:04-08:00 [INFO] Live MAC table returned 2112 entries for switch9
2026-02-13T12:42:04-08:00 [INFO] Querying switch: switch10 (XXXX-XXXX-XXXX)
2026-02-13T12:42:06-08:00 [INFO] Live MAC table returned 65 entries for switch10
2026-02-13T12:42:06-08:00 [INFO] Querying switch: switch11 (XXXX-XXXX-XXXX)
2026-02-13T12:42:09-08:00 [INFO] Live MAC table returned 2592 entries for switch11
2026-02-13T12:42:09-08:00 [INFO] Network: Network3
2026-02-13T12:42:14-08:00 [INFO] Network clients API returned 1000 clients
2026-02-13T12:42:14-08:00 [INFO] Querying switch: switch12 (XXXX-XXXX-XXXX)
2026-02-13T12:42:17-08:00 [INFO] Live MAC table returned 2163 entries for switch12
2026-02-13T12:42:17-08:00 [INFO] Querying switch: switch13 (XXXX-XXXX-XXXX)
2026-02-13T12:42:52-08:00 [INFO] Device clients API returned 0 clients for switch13
2026-02-13T12:42:52-08:00 [INFO] Querying switch: switch14 (XXXX-XXXX-XXXX)
2026-02-13T12:43:26-08:00 [INFO] Device clients API returned 0 clients for switch14
2026-02-13T12:43:26-08:00 [INFO] Querying switch: switch15 (XXXX-XXXX-XXXX)
2026-02-13T12:43:29-08:00 [INFO] Live MAC table returned 2025 entries for switch15
2026-02-13T12:43:29-08:00 [INFO] Querying switch: switch16 (XXXX-XXXX-XXXX)
2026-02-13T12:43:32-08:00 [INFO] Live MAC table returned 2017 entries for switch16
2026-02-13T12:43:32-08:00 [INFO] Querying switch: switch17 (XXXX-XXXX-XXXX)
2026-02-13T12:43:35-08:00 [INFO] Live MAC table returned 1175 entries for switch17
2026-02-13T12:43:35-08:00 [INFO] Querying switch: switch18 (XXXX-XXXX-XXXX)
2026-02-13T12:43:37-08:00 [INFO] Live MAC table returned 2060 entries for switch18
2026-02-13T12:43:37-08:00 [INFO] Querying switch: switch19 (XXXX-XXXX-XXXX)
2026-02-13T12:43:40-08:00 [INFO] Live MAC table returned 2088 entries for switch19
2026-02-13T12:43:40-08:00 [INFO] Querying switch: switch20 (XXXX-XXXX-XXXX)
2026-02-13T12:44:15-08:00 [INFO] Device clients API returned 0 clients for switch20
2026-02-13T12:44:15-08:00 [INFO] Querying switch: switch21 (XXXX-XXXX-XXXX)
2026-02-13T12:44:18-08:00 [INFO] Live MAC table returned 1185 entries for switch21
2026-02-13T12:44:18-08:00 [INFO] Querying switch: switch22 (XXXX-XXXX-XXXX)
2026-02-13T12:44:20-08:00 [INFO] Live MAC table returned 2127 entries for switch22
2026-02-13T12:44:20-08:00 [INFO] Querying switch: switch23 (XXXX-XXXX-XXXX)
2026-02-13T12:44:23-08:00 [INFO] Live MAC table returned 59 entries for switch23
2026-02-13T12:44:23-08:00 [INFO] Querying switch: switch24 (XXXX-XXXX-XXXX)
2026-02-13T12:44:26-08:00 [INFO] Live MAC table returned 2108 entries for switch24
2026-02-13T12:44:26-08:00 [INFO] Network: Network4
2026-02-13T12:44:30-08:00 [INFO] Network clients API returned 1000 clients
2026-02-13T12:44:30-08:00 [INFO] Querying switch: switch25 (XXXX-XXXX-XXXX)
2026-02-13T12:44:32-08:00 [INFO] Live MAC table returned 2054 entries for switch25
2026-02-13T12:44:32-08:00 [INFO] Querying switch: switch26 (XXXX-XXXX-XXXX)
2026-02-13T12:44:35-08:00 [INFO] Live MAC table returned 2051 entries for switch26
2026-02-13T12:44:35-08:00 [INFO] Network: Network5
2026-02-13T12:44:38-08:00 [INFO] Network clients API returned 556 clients
2026-02-13T12:44:38-08:00 [INFO] Querying switch: switch27 (XXXX-XXXX-XXXX)
2026-02-13T12:44:40-08:00 [INFO] Live MAC table returned 335 entries for switch27
2026-02-13T12:44:40-08:00 [INFO] Querying switch: switch28 (XXXX-XXXX-XXXX)
2026-02-13T12:44:43-08:00 [INFO] Live MAC table returned 24 entries for switch28
2026-02-13T12:44:43-08:00 [INFO] Network: Network6
2026-02-13T12:44:47-08:00 [INFO] Network clients API returned 1000 clients
2026-02-13T12:44:47-08:00 [INFO] Querying switch: switch29 (XXXX-XXXX-XXXX)
2026-02-13T12:44:52-08:00 [INFO] Live MAC table returned 2623 entries for switch29
2026-02-13T12:44:52-08:00 [INFO] Querying switch: switch30 (XXXX-XXXX-XXXX)
2026-02-13T12:44:57-08:00 [INFO] Live MAC table returned 1660 entries for switch30
2026-02-13T12:44:57-08:00 [INFO] Querying switch: switch31 (XXXX-XXXX-XXXX)
2026-02-13T12:45:00-08:00 [INFO] Live MAC table returned 1061 entries for switch31
2026-02-13T12:45:00-08:00 [INFO] Querying switch: switch32 (XXXX-XXXX-XXXX)
2026-02-13T12:45:02-08:00 [INFO] Live MAC table returned 2589 entries for switch32
2026-02-13T12:45:02-08:00 [INFO] Querying switch: switch33 (XXXX-XXXX-XXXX)
2026-02-13T12:45:37-08:00 [INFO] Device clients API returned 2 clients for switch33
2026-02-13T12:45:37-08:00 [INFO] Querying switch: switch34 (XXXX-XXXX-XXXX)
2026-02-13T12:45:40-08:00 [INFO] Live MAC table returned 2614 entries for switch34
2026-02-13T12:45:40-08:00 [INFO] Network: Network7
2026-02-13T12:45:42-08:00 [INFO] Network clients API returned 51 clients
2026-02-13T12:45:42-08:00 [INFO] Querying switch: switch35 (XXXX-XXXX-XXXX)
2026-02-13T12:45:44-08:00 [INFO] Live MAC table returned 5 entries for switch35
2026-02-13T12:45:44-08:00 [INFO] Network: Network8
2026-02-13T12:45:46-08:00 [INFO] Network clients API returned 154 clients
2026-02-13T12:45:46-08:00 [INFO] Querying switch: switch36 (XXXX-XXXX-XXXX)
2026-02-13T12:45:49-08:00 [INFO] Live MAC table returned 10 entries for switch36
2026-02-13T12:45:49-08:00 [INFO] Network: Network9
2026-02-13T12:45:51-08:00 [INFO] Network clients API returned 58 clients
2026-02-13T12:45:51-08:00 [INFO] Querying switch: switch37 (XXXX-XXXX-XXXX)
2026-02-13T12:45:53-08:00 [INFO] Live MAC table returned 7 entries for switch37
2026-02-13T12:45:53-08:00 [INFO] Network: Network10
2026-02-13T12:45:55-08:00 [INFO] Network clients API returned 88 clients
2026-02-13T12:45:55-08:00 [INFO] Querying switch: switch38 (XXXX-XXXX-XXXX)
2026-02-13T12:45:57-08:00 [INFO] Live MAC table returned 8 entries for switch38
2026-02-13T12:45:57-08:00 [INFO] Network: Network11
2026-02-13T12:45:59-08:00 [INFO] Network clients API returned 0 clients
2026-02-13T12:45:59-08:00 [INFO] Network: Network12
2026-02-13T12:45:59-08:00 [INFO] Network clients API returned 0 clients
2026-02-13T12:45:59-08:00 [INFO] Network: Network13
2026-02-13T12:46:01-08:00 [INFO] Network clients API returned 194 clients
2026-02-13T12:46:01-08:00 [INFO] Network: Network14
2026-02-13T12:46:02-08:00 [INFO] Network clients API returned 54 clients
2026-02-13T12:46:02-08:00 [INFO] Network: Network15
2026-02-13T12:46:04-08:00 [INFO] Network clients API returned 137 clients
2026-02-13T12:46:04-08:00 [INFO] Network: Network16
2026-02-13T12:46:05-08:00 [INFO] Network clients API returned 0 clients
2026-02-13T12:46:05-08:00 [INFO] Network: Lab
2026-02-13T12:46:06-08:00 [INFO] Network clients API returned 74 clients
2026-02-13T12:46:06-08:00 [INFO] Network: Network17
2026-02-13T12:46:07-08:00 [INFO] Network clients API returned 4 clients
Org,Network,Switch,Serial,Port,MAC,LastSeen
My Organization,Network1,switch6,XXXX-XXXX-XXXX,51,00:11:22:33:44:55,
My Organization,Network5,switch27,XXXX-XXXX-XXXX,54,00:11:22:33:44:55,2026-02-13T15:24:38Z
```