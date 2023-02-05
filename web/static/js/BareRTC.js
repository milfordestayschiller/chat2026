console.log("BareRTC!");

// WebRTC configuration.
const configuration = {
    iceServers: [{
        urls: 'stun:stun.l.google.com:19302'
    }]
};


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

            // WebRTC sessions with other users.
            WebRTC: {
                // Streams per username.
                streams: {},

                // RTCPeerConnections per username.
                pc: {},
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

        /**
         * Chat API Methods (WebSocket packets sent/received)
         */

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
                this.username = msg.username;
            }

            this.ChatClient(`User sync from backend: ${JSON.stringify(msg)}`);
        },

        // Send a video request to access a user's camera.
        sendOpen(username) {
            this.ws.conn.send(JSON.stringify({
                action: "open",
                username: username,
            }));
        },
        onOpen(msg) {
            // Response for the opener to begin WebRTC connection.
            const secret = msg.openSecret;
            console.log("OPEN: connect to %s with secret %s", msg.username, secret);
            this.ChatClient(`onOpen called for ${msg.username}.`);

            this.startWebRTC(msg.username, true);
        },
        onRing(msg) {
            // Message for the receiver to begin WebRTC connection.
            const secret = msg.openSecret;
            console.log("RING: connection from %s with secret %s", msg.username, secret);
            this.ChatServer(`${msg.username} has opened your camera.`);

            this.startWebRTC(msg.username, false);
        },
        onUserExited(msg) {
            // A user has logged off the server. Clean up any WebRTC connections.
            delete(this.WebRTC.streams[msg.username]);
            delete(this.WebRTC.pc[msg.username]);
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
            console.log("Dialing WebSocket...");
            const proto = location.protocol === 'https:' ? 'wss' : 'ws';
            const conn = new WebSocket(`${proto}://${location.host}/ws`);

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
                        // TODO: make a dedicated leave event
                        if (msg.message.indexOf("has exited the room!") > -1) {
                            // Clean up data about this user.
                            this.onUserExited(msg);
                        }
                        this.pushHistory({
                            action: msg.action,
                            username: msg.username,
                            message: msg.message,
                        });
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
                    case "error":
                        this.pushHistory({
                            username: msg.username || 'Internal Server Error',
                            message: msg.message,
                            isChatServer: true,
                        });
                    default:
                        console.error("Unexpected action: %s", JSON.stringify(msg));
                }
            });

            this.ws.conn = conn;
        },

        /**
         * WebRTC concerns.
         */

        // Start WebRTC with the other username.
        startWebRTC(username, isOfferer) {
            this.ChatClient(`Begin WebRTC with ${username}.`);
            let pc = new RTCPeerConnection(configuration);
            this.WebRTC.pc[username] = pc;

            // Create a data channel so we have something to connect over even if
            // the local user is not broadcasting their own camera.
            let dataChannel = pc.createDataChannel("data");
            dataChannel.addEventListener("open", event => {
                // beginTransmission(dataChannel);
            })

            // 'onicecandidate' notifies us whenever an ICE agent needs to deliver a
            // message to the other peer through the signaling server.
            pc.onicecandidate = event => {
                console.error("WebRTC OnICECandidate called!", event);
                // this.ChatClient("On ICE Candidate called!");
                if (event.candidate) {
                    // this.ChatClient(`Send ICE candidate: ${JSON.stringify(event.candidate)}`);
                    console.log("Sending candidate to websockets:", event.candidate);
                    this.ws.conn.send(JSON.stringify({
                        action: "candidate",
                        username: username,
                        candidate: event.candidate,
                    }));
                }
            };

            // If the user is offerer let the 'negotiationneeded' event create the offer.
            if (isOfferer) {
                this.ChatClient("We are the offerer - set up onNegotiationNeeded");
                pc.onnegotiationneeded = () => {
                    console.error("WebRTC OnNegotiationNeeded called!");
                    this.ChatClient("Negotiation Needed, creating WebRTC offer.");
                    pc.createOffer().then(this.localDescCreated(pc, username)).catch(this.ChatClient);
                };
            }

            // When a remote stream arrives.
            pc.ontrack = event => {
                this.ChatServer("ON TRACK CALLED!!!");
                console.error("WebRTC OnTrack called!", event);
                const stream = event.streams[0];

                // Do we already have it?
                this.ChatClient(`Received a video stream from ${username}.`);
                if (this.WebRTC.streams[username] == undefined ||
                    this.WebRTC.streams[username].id !== stream.id) {
                    this.WebRTC.streams[username] = stream;
                }

                window.requestAnimationFrame(() => {
                    this.ChatServer("Setting <video> srcObject for "+username);
                    let $ref = document.getElementById(`videofeed-${username}`);
                    console.log("Video elem:", $ref);
                    $ref.srcObject = stream;
                    // this.$refs[`videofeed-${username}`].srcObject = stream;
                });
            };

            // If we were already broadcasting video, send our stream to
            // the connecting user.
            // TODO: currently both users need to have video on for the connection
            // to succeed - if offerer doesn't addTrack it won't request a video channel
            // and so the answerer (who has video) won't actually send its
            if (this.webcam.active) {
                this.ChatClient(`Sharing our video stream to ${username}.`);
                let stream = this.webcam.stream;
                stream.getTracks().forEach(track => {
                    console.error("Add stream track to WebRTC", stream, track);
                    pc.addTrack(track, stream)
                });
            }

            // If we are the offerer, begin the connection.
            if (isOfferer) {
                pc.createOffer().then(this.localDescCreated(pc, username)).catch(this.ChatClient);
            }
        },

        // Common handler function for
        localDescCreated(pc, username) {
            return (desc) => {
                this.ChatClient(`setLocalDescription ${JSON.stringify(desc)}`);
                pc.setLocalDescription(
                    new RTCSessionDescription(desc),
                    () => {
                        this.ws.conn.send(JSON.stringify({
                            action: "sdp",
                            username: username,
                            description: pc.localDescription,
                        }));
                    },
                    console.error,
                )
            };
        },

        // Handle inbound WebRTC signaling messages proxied by the websocket.
        onCandidate(msg) {
            console.error("onCandidate() called:", msg);
            if (this.WebRTC.pc[msg.username] == undefined) {
                console.error("DID NOT FIND RTCPeerConnection for username:", msg.username);
                return;
            }
            let pc = this.WebRTC.pc[msg.username];

            // Add the new ICE candidate.
            console.log("Add ICE candidate: %s", msg.candidate);
            // this.ChatClient(`Received an ICE candidate from ${msg.username}: ${JSON.stringify(msg.candidate)}`);
            pc.addIceCandidate(
                new RTCIceCandidate(
                    msg.candidate,
                    () => {},
                    console.error,
                )
            );
        },
        onSDP(msg) {
            console.error("onSDP() called:", msg);
            if (this.WebRTC.pc[msg.username] == undefined) {
                console.error("DID NOT FIND RTCPeerConnection for username:", msg.username);
                return;
            }
            let pc = this.WebRTC.pc[msg.username];
            let message = msg.description;

            // Add the new ICE candidate.
            console.log("Set description:", message);
            this.ChatClient(`Received a Remote Description from ${msg.username}: ${JSON.stringify(msg.description)}.`);
            pc.setRemoteDescription(new RTCSessionDescription(message), () => {
                // When receiving an offer let's answer it.
                if (pc.remoteDescription.type === 'offer') {
                    console.error("Webcam:", this.webcam);

                    // Add our local video tracks to the connection.
                    // if (this.webcam.active) {
                    //     this.ChatClient(`Sharing our video stream to ${msg.username}.`);
                    //     let stream = this.webcam.stream;
                    //     stream.getTracks().forEach(track => {
                    //         console.error("Add stream track to WebRTC", stream, track);
                    //         pc.addTrack(track, stream)
                    //     });
                    // }

                    this.ChatClient(`setRemoteDescription callback. Offer recieved - sending answer. Cam active? ${this.webcam.active}`);
                    console.warn("Creating answer now");
                    pc.createAnswer().then(this.localDescCreated(pc, msg.username)).catch(this.ChatClient);
                }
            }, console.error);
        },

        /**
         * Front-end web app concerns.
         */

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

        // Begin connecting to someone else's webcam.
        openVideo(user) {
            if (user.username === this.username) {
                this.ChatClient("You can already see your own webcam.");
                return;
            }

            // We need to broadcast video to connect to another.
            // TODO: because if the offerer doesn't add video tracks they
            // won't request video support so the answerer's video isn't sent
            if (!this.webcam.active) {
                this.ChatServer("You will need to turn your own camera on first before you can connect to " + user.username + ".");
                return;
            }

            this.sendOpen(user.username);
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