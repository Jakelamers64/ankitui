# ankitui

How to setup ankiconnect connection from other device on network

To connect to an AnkiConnect HTTP server from another computer on your local network, follow these steps:

1. **Find the IP address of the computer hosting AnkiConnect**
   - On Windows: Open Command Prompt and type `ipconfig`
   - On macOS/Linux: Open Terminal and type `ifconfig` or `ip addr`
   - Look for your IPv4 address (typically starts with 192.168, 10.0, or 172)

2. **Configure AnkiConnect to accept connections from other computers**
   - Open Anki on the host computer
   - Go to Tools → Add-ons → AnkiConnect → Config
   - Change the "host" setting from "127.0.0.1" (localhost) to "0.0.0.0" (all interfaces)
   - Adjust the "webBindAddress" if present to "0.0.0.0" as well
   - Example config:
     ```json
     {
       "host": "0.0.0.0",
       "port": 8765,
       "webBindAddress": "0.0.0.0",
       "webCorsOriginList": ["*"]
     }
     ```
   - Restart Anki for changes to take effect

3. **Check your firewall settings**
   - Make sure your host computer's firewall allows incoming connections on the AnkiConnect port (default 8765)
   - Add an exception for Anki or the specific port if needed

4. **Connect from the other computer**
   - Use the host computer's IP address and the AnkiConnect port
   - For example, if the host IP is 192.168.1.100, you would connect to:
     `http://192.168.1.100:8765`

5. **Test the connection**
   - From the client computer, you can test with a simple HTTP request:
   - Using curl: `curl -X POST -d '{"action": "version", "version": 6}' http://192.168.1.100:8765`
   - Or using a web browser, navigate to `http://192.168.1.100:8765`

Is there a specific application or programming language you're using to connect to AnkiConnect that you need help with?
