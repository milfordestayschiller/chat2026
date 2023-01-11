console.log("BareRTC!");

const app = Vue.createApp({
    delimiters: ['[[', ']]'],
    data() {
        return {
            busy: false,

            username: "",
            message: "",

            // WebSocket connection.
            ws: {
                conn: null,
                connected: false,
            },

            // Chat history.
            history: [],
            DMs: {},

            loginModal: {
                visible: false,
            },
        }
    },
    mounted() {
        this.pushHistory({
            username: "ChatServer",
            message: "Welcome to BareRTC!",
        });

        if (!this.username) {
            this.loginModal.visible = true;
        }
    },
    methods: {
        signIn() {
            this.loginModal.visible = false;
            this.dial();
        },

        sendMessage() {
            if (!this.message) {
                return;
            }

            if (!this.ws.connected) {
                this.pushHistory({
                    username: "ChatClient",
                    message: "You are not connected to the server.",
                });
                return;
            }

            console.debug("Send message: %s", this.message);
            this.ws.conn.send(JSON.stringify({
                action: "message",
                message: this.message,
            }));

            this.message = "";
        },

        // Dial the WebSocket connection.
        dial() {
            const conn = new WebSocket(`ws://${location.host}/ws`);

            conn.addEventListener("close", ev => {
                this.ws.connected = false;
                this.pushHistory({
                    username: "ChatClient",
                    message: `WebSocket Disconnected code: ${ev.code}, reason: ${ev.reason}`,
                });

                if (ev.code !== 1001) {
                    this.pushHistory({
                        username: "ChatClient",
                        message: "Reconnecting in 1s",
                    });
                    setTimeout(this.dial, 1000);
                }
            });

            conn.addEventListener("open", ev => {
                this.ws.connected = true;
                this.pushHistory({
                    username: "ChatClient",
                    message: "Websocket connected!",
                });

                // Tell the server our username.
                this.ws.conn.send(JSON.stringify({
                    action: "login",
                    username: this.username,
                }));
            });

            conn.addEventListener("message", ev => {
                console.log(ev);
                if (typeof ev.data !== "string") {
                    console.error("unexpected message type", typeof ev.data);
                    return;
                }

                let msg = JSON.parse(ev.data);

                this.pushHistory({
                    username: msg.username,
                    message: msg.message,
                });
            });

            this.ws.conn = conn;
        },

        pushHistory({username, message}) {
            this.history.push({
                username: username,
                message: message,
            });
        }
    }
});

app.mount("#BareRTC-App");