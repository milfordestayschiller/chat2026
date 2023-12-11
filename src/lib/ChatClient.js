// WebSocket chat client handler.
class ChatClient {
    /**
     * Constructor for the client.
     *
     * @param usePolling: instead of WebSocket use the ajax polling API.
     * @param onClientError: function to receive 'ChatClient' messages to
     *                       add to the chat room (this.ChatClient())
     */
    constructor({
        usePolling=false,
        onClientError,

        jwt,   // JWT token for authorization
        prefs, // User preferences for 'me' action (close DMs, etc)

        // Chat Protocol handler functions for the caller.
        onWho,
        onMe,
        onMessage,
        onTakeback,
        onReact,
        onPresence,
        onRing,
        onOpen,
        onCandidate,
        onSDP,
        onWatch,
        onUnwatch,
        onBlock,

        // Misc function registrations for callback.
        onNewJWT, // new JWT token from ping response
        bulkMuteUsers, // Upload our blocklist on connect.
        focusMessageBox, // Tell caller to focus the message entry box.
        pushHistory,
     }) {
        this.usePolling = usePolling;

        // Pointer to the 'ChatClient(message)' command from the main app.
        this.ChatClient = onClientError;

        this.jwt = jwt;
        this.prefs = prefs;

        // Register the handler functions.
        this.onWho = onWho;
        this.onMe = onMe;
        this.onMessage = onMessage;
        this.onTakeback = onTakeback;
        this.onReact = onReact;
        this.onPresence = onPresence;
        this.onRing = onRing;
        this.onOpen = onOpen;
        this.onCandidate = onCandidate;
        this.onSDP = onSDP;
        this.onWatch = onWatch;
        this.onUnwatch = onUnwatch;
        this.onBlock = onBlock;

        this.onNewJWT = onNewJWT;
        this.bulkMuteUsers = bulkMuteUsers;
        this.focusMessageBox = focusMessageBox;
        this.pushHistory = pushHistory;

        // WebSocket connection.
        this.ws = {
            conn: null,
            connected: false,
        };
    }

    // Connected polls if the client is connected.
    connected() {
        if (this.usePolling) {
            return true;
        }
        return this.ws.connected;
    }

    // Disconnect from the server.
    disconnect() {
        if (this.usePolling) {
            throw new Exception("Not implemented");
        }
        this.ws.conn.close();
    }

    // Common function to send a message to the server. The message
    // is a JSON object before stringify.
    send(message) {
        if (this.usePolling) {
            throw new Exception("Not implemented");
        }

        if (!this.ws.connected) {
            this.ChatClient("Couldn't send WebSocket message: not connected.");
            return;
        }

        console.log("send:", message);
        if (typeof(message) !== "string") {
            message = JSON.stringify(message);
        }
        this.ws.conn.send(message);
    }

    // Common function to handle a message from the server.
    handle(msg) {
        switch (msg.action) {
            case "who":
                this.onWho(msg);
                break;
            case "me":
                this.onMe(msg);
                break;
            case "message":
                this.onMessage(msg);
                break;
            case "takeback":
                this.onTakeback(msg);
                break;
            case "react":
                this.onReact(msg);
                break;
            case "presence":
                this.onPresence(msg);
                break;
            case "ring":
                this.onRing(msg);
                break;
            case "open":
                this.onOpen(msg);
                break;
            case "candidate":
                this.onCandidate(msg);
                break;
            case "sdp":
                this.onSDP(msg);
                break;
            case "watch":
                this.onWatch(msg);
                break;
            case "unwatch":
                this.onUnwatch(msg);
                break;
            case "block":
                this.onBlock(msg);
                break;
            case "error":
                this.pushHistory({
                    channel: msg.channel,
                    username: msg.username || 'Internal Server Error',
                    message: msg.message,
                    isChatServer: true,
                });
                break;
            case "disconnect":
                this.onWho({ whoList: [] });
                this.disconnect = true;
                this.ws.connected = false;
                this.ws.conn.close(1000, "server asked to close the connection");
                break;
            case "ping":
                // New JWT token?
                if (msg.jwt) {
                    this.onNewJWT(msg.jwt);
                }

                // Reset disconnect retry counter: if we were on long enough to get
                // a ping, we're well connected and can reconnect no matter how many
                // times the chat server is rebooted.
                this.disconnectCount = 0;
                break;
            default:
                console.error("Unexpected action: %s", JSON.stringify(msg));
        }
    }

    // Dial the WebSocket.
    dial() {
        this.ChatClient("Establishing connection to server...");

        const proto = location.protocol === 'https:' ? 'wss' : 'ws';
        const conn = new WebSocket(`${proto}://${location.host}/ws`);

        conn.addEventListener("close", ev => {
            // Lost connection to server - scrub who list.
            this.onWho({ whoList: [] });

            this.ws.connected = false;
            this.ChatClient(`WebSocket Disconnected code: ${ev.code}, reason: ${ev.reason}`);

            this.disconnectCount++;
            if (this.disconnectCount > this.disconnectLimit) {
                this.ChatClient(`It seems there's a problem connecting to the server. Please try some other time.`);
                return;
            }

            if (!this.disconnect) {
                if (ev.code !== 1001 && ev.code !== 1000) {
                    this.ChatClient("Reconnecting in 5s");
                    setTimeout(this.dial, 5000);
                }
            }
        });

        conn.addEventListener("open", ev => {
            this.ws.connected = true;
            this.ChatClient("Websocket connected!");

            // Upload our blocklist to the server before login. This resolves a bug where if a block
            // was added recently (other user still online in chat), that user would briefly see your
            // "has entered the room" message followed by you immediately not being online.
            this.bulkMuteUsers();

            // Tell the server our username.
            this.send({
                action: "login",
                username: this.username,
                jwt: this.jwt.token,
                dnd: this.prefs.closeDMs,
            });

            // Focus the message entry box.
            window.requestAnimationFrame(() => {
                this.focusMessageBox();
            });
        });

        conn.addEventListener("message", ev => {
            if (typeof ev.data !== "string") {
                console.error("unexpected message type", typeof ev.data);
                return;
            }

            let msg = JSON.parse(ev.data);
            this.handle(msg);
        });

        this.ws.conn = conn;
    }
}

export default ChatClient;
