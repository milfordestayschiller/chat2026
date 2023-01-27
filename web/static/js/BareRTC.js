console.log("BareRTC!");

const app = Vue.createApp({
    delimiters: ['[[', ']]'],
    data() {
        return {
            busy: false,

            channel: "lobby",
            username: "", //"test",
            message: "",

            // WebSocket connection.
            ws: {
                conn: null,
                connected: false,
            },

            // Who List for the room.
            whoList: [],

            // My video feed.
            webcam: {
                busy: false,
                active: false,
                elem: null,   // <video id="localVideo"> element
                stream: null, // MediaStream object
            },

            // Chat history.
            history: [],
            historyScrollbox: null,
            DMs: {},

            loginModal: {
                visible: false,
            },
        }
    },
    mounted() {
        this.webcam.elem = document.querySelector("#localVideo");
        this.historyScrollbox = document.querySelector("#chatHistory");

        this.ChatServer("Welcome to BareRTC!")

        if (!this.username) {
            this.loginModal.visible = true;
        } else {
            this.signIn();
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
                this.ChatClient("You are not connected to the server.");
                return;
            }

            console.debug("Send message: %s", this.message);
            this.ws.conn.send(JSON.stringify({
                action: "message",
                message: this.message,
            }));

            this.message = "";
        },

        // Sync the current user state (such as video broadcasting status) to
        // the backend, which will reload everybody's Who List.
        sendMe() {
            this.ws.conn.send(JSON.stringify({
                action: "me",
                videoActive: this.webcam.active,
            }));
        },
        onMe(msg) {
            // We have had settings pushed to us by the server, such as a change
            // in our choice of username.
            if (this.username != msg.username) {
                this.ChatServer(`Your username has been changed to ${msg.username}.`);
            }

            this.ChatClient(`User sync from backend: ${JSON.stringify(msg)}`);
        },

        // Handle messages sent in chat.
        onMessage(msg) {
            this.pushHistory({
                username: msg.username,
                message: msg.message,
            });
        },

        // Dial the WebSocket connection.
        dial() {
            const conn = new WebSocket(`ws://${location.host}/ws`);

            conn.addEventListener("close", ev => {
                this.ws.connected = false;
                this.ChatClient(`WebSocket Disconnected code: ${ev.code}, reason: ${ev.reason}`);

                if (ev.code !== 1001) {
                    this.ChatClient("Reconnecting in 1s");
                    setTimeout(this.dial, 1000);
                }
            });

            conn.addEventListener("open", ev => {
                this.ws.connected = true;
                this.ChatClient("Websocket connected!");

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
                switch (msg.action) {
                    case "who":
                        console.log("Got the Who List: %s", msg);
                        this.whoList = msg.whoList;
                        break;
                    case "me":
                        console.log("Got a self-update: %s", msg);
                        this.onMe(msg);
                        break;
                    case "message":
                        this.onMessage(msg);
                        break;
                    case "presence":
                        this.pushHistory({
                            action: msg.action,
                            username: msg.username,
                            message: msg.message,
                        });
                        break;
                    default:
                        console.error("Unexpected action: %s", JSON.stringify(msg));
                }
            });

            this.ws.conn = conn;
        },

        // Start broadcasting my webcam.
        startVideo() {
            if (this.webcam.busy) return;
            this.webcam.busy = true;

            navigator.mediaDevices.getUserMedia({
                audio: true,
                video: true,
            }).then(stream => {
                this.webcam.active = true;
                this.webcam.elem.srcObject = stream;
                this.webcam.stream = stream;

                // Tell backend the camera is ready.
                this.sendMe();
            }).catch(err => {
                this.ChatClient(`Webcam error: ${err}`);
            }).finally(() => {
                this.webcam.busy = false;
            })
        },

        // Stop broadcasting.
        stopVideo() {
            this.webcam.elem.srcObject = null;
            this.webcam.stream = null;
            this.webcam.active = false;

            // Tell backend our camera state.
            this.sendMe();
        },

        pushHistory({username, message, action="message", isChatServer, isChatClient}) {
            this.history.push({
                action: action,
                username: username,
                message: message,
                isChatServer,
                isChatClient,
            });
            this.scrollHistory();
        },

        scrollHistory() {
            window.requestAnimationFrame(() => {
                this.historyScrollbox.scroll({
                    top: this.historyScrollbox.scrollHeight,
                    behavior: 'smooth',
                });
            });

        },

        // Send a chat message as ChatServer
        ChatServer(message) {
            this.pushHistory({
                username: "ChatServer",
                message: message,
                isChatServer: true,
            });
        },
        ChatClient(message) {
            this.pushHistory({
                username: "ChatClient",
                message: message,
                isChatClient: true,
            });
        },
    }
});

app.mount("#BareRTC-App");