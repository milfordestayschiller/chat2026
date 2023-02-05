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

            // Website configuration provided by chat.html template.
            config: {
                channels: PublicChannels,
            },

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
            channels: {
                // There will be values here like:
                // "lobby": {
                //   "history": [],
                //   "updated": timestamp,
                //   "unread": 4,
                // },
                // "@username": {
                //   "history": [],
                //   ...
                // }
            },
            historyScrollbox: null,
            DMs: {},

            // Responsive CSS for mobile.
            responsive: {
                leftDrawerOpen: false,
                rightDrawerOpen: false,
                nodes: {
                    // DOM nodes for the CSS grid cells.
                    $container: null,
                    $left: null,
                    $center: null,
                    $right: null,
                }
            },

            loginModal: {
                visible: false,
            },
        }
    },
    mounted() {
        this.webcam.elem = document.querySelector("#localVideo");
        this.historyScrollbox = document.querySelector("#chatHistory");

        this.responsive.nodes = {
            $container: document.querySelector(".chat-container"),
            $left: document.querySelector(".left-column"),
            $center: document.querySelector(".chat-column"),
            $right: document.querySelector(".right-column"),
        };

        window.addEventListener("resize", () => {
            // Reset CSS overrides for responsive display on any window size change.
            this.resetResponsiveCSS();
        });

        for (let channel of this.config.channels) {
            this.initHistory(channel.ID);
        }

        this.ChatServer("Welcome to BareRTC!")

        if (!this.username) {
            this.loginModal.visible = true;
        } else {
            this.signIn();
        }
    },
    computed: {
        chatHistory() {
            if (this.channels[this.channel] == undefined) {
                return [];
            }

            let history = this.channels[this.channel].history;

            // How channels work:
            // - Everything going to a public channel like "lobby" goes
            //   into the "lobby" channel in the front-end
            // - Direct messages are different: they are all addressed
            //   "to" the channel of the current @user, but they are
            //   divided into DM threads based on the username.
            if (this.channel.indexOf("@") === 0) {
                // DM thread, divide them by sender.
                // let username = this.channel.substring(1);
                // return history.filter(v => {
                //     return v.username === username;
                // });
            }
            return history;
        },
        channelName() {
            // Return a suitable channel title.
            if (this.channel.indexOf("@") === 0) {
                // A DM, return it directly as is.
                return this.channel;
            }

            // Find the friendly name from our config.
            for (let channel of this.config.channels) {
                if (channel.ID === this.channel) {
                    return channel.Name;
                }
            }

            return this.channel;
        },
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
                channel: this.channel,
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

            // this.ChatClient(`User sync from backend: ${JSON.stringify(msg)}`);
        },

        // WhoList updates.
        onWho(msg) {
            this.whoList = msg.whoList;

            // If we had a camera open with any of these and they have gone
            // off camera, close our side of the connection.
            for (let row of this.whoList) {
                if (this.WebRTC.streams[row.username] != undefined &&
                    row.videoActive !== true) {
                    this.closeVideo(row.username);
                }
            }
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
            // this.ChatClient(`onOpen called for ${msg.username}.`);

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
            this.closeVideo(msg.username);
        },

        // Handle messages sent in chat.
        onMessage(msg) {
            this.pushHistory({
                channel: msg.channel,
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
                        this.onWho(msg);
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
                            isChatClient: true,
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
            // this.ChatClient(`Begin WebRTC with ${username}.`);
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
                // this.ChatClient("We are the offerer - set up onNegotiationNeeded");
                pc.onnegotiationneeded = () => {
                    console.error("WebRTC OnNegotiationNeeded called!");
                    this.ChatClient("Negotiation Needed, creating WebRTC offer.");
                    pc.createOffer().then(this.localDescCreated(pc, username)).catch(this.ChatClient);
                };
            }

            // When a remote stream arrives.
            pc.ontrack = event => {
                // this.ChatServer("ON TRACK CALLED!!!");
                console.error("WebRTC OnTrack called!", event);
                const stream = event.streams[0];

                // Do we already have it?
                // this.ChatClient(`Received a video stream from ${username}.`);
                if (this.WebRTC.streams[username] == undefined ||
                    this.WebRTC.streams[username].id !== stream.id) {
                    this.WebRTC.streams[username] = stream;
                }

                window.requestAnimationFrame(() => {
                    this.ChatServer("Setting <video> srcObject for " + username);
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
            if (!isOfferer && this.webcam.active) {
                // this.ChatClient(`Sharing our video stream to ${username}.`);
                let stream = this.webcam.stream;
                stream.getTracks().forEach(track => {
                    console.error("Add stream track to WebRTC", stream, track);
                    pc.addTrack(track, stream)
                });
            }

            // If we are the offerer, begin the connection.
            if (isOfferer) {
                pc.createOffer({
                    offerToReceiveVideo: true,
                    offerToReceiveAudio: true,
                }).then(this.localDescCreated(pc, username)).catch(this.ChatClient);
            }
        },

        // Common handler function for
        localDescCreated(pc, username) {
            return (desc) => {
                // this.ChatClient(`setLocalDescription ${JSON.stringify(desc)}`);
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
                    () => { },
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
            // this.ChatClient(`Received a Remote Description from ${msg.username}: ${JSON.stringify(msg.description)}.`);
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

                    // this.ChatClient(`setRemoteDescription callback. Offer recieved - sending answer. Cam active? ${this.webcam.active}`);
                    console.warn("Creating answer now");
                    pc.createAnswer().then(this.localDescCreated(pc, msg.username)).catch(this.ChatClient);
                }
            }, console.error);
        },

        /**
         * Front-end web app concerns.
         */

        // Set active chat room.
        setChannel(channel) {
            this.channel = typeof(channel) === "string" ? channel : channel.ID;
            this.scrollHistory();
            this.channels[this.channel].unread = 0;
        },
        hasUnread(channel) {
            if (this.channels[channel] == undefined) {
                return 0;
            }
            return this.channels[channel].unread;
        },
        openDMs(user) {
            let channel = "@" + user.username;
            this.initHistory(channel);
            this.setChannel(channel);
        },
        leaveDM() {
            // Validate we're in a DM currently.
            if (this.channel.indexOf("@") !== 0) return;

            if (!window.confirm(
                "Do you want to close this chat thread? Your conversation history will " +
                "be forgotten on your computer, but your chat partner may still have " +
                "your chat thread open on their end."
            )) {
                return;
            }

            let channel = this.channel;
            this.setChannel(this.config.channels[0].ID);
            delete(this.channels[channel]);
        },

        activeChannels() {
            // List of current channels, unread indicators etc.
            let result = [];
            for (let channel of this.config.channels) {
                let data = {
                    ID: channel.ID,
                    Name: channel.Name,
                };
                if (this.channels[channel] != undefined) {
                    data.Unread = this.channels[channel].unread;
                    data.Updated = this.channels[channel].updated;
                }
                result.push(data);
            }
            return result;
        },
        activeDMs() {
            // List your currently open DM threads, sorted by most recent.
            let result = [];
            for (let channel of Object.keys(this.channels)) {
                // @mentions only
                if (channel.indexOf("@") !== 0) {
                    continue;
                }

                result.push({
                    channel: channel,
                    name: channel.substring(1),
                    updated: this.channels[channel].updated,
                    unread: this.channels[channel].unread,
                });
            }

            result.sort((a, b) => {
                return a.updated < b.updated;
            });
            return result;
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

        // Begin connecting to someone else's webcam.
        openVideo(user) {
            if (user.username === this.username) {
                this.ChatClient("You can already see your own webcam.");
                return;
            }

            // Camera is already open? Then disconnect the connection.
            if (this.WebRTC.pc[user.username] != undefined) {
                // TODO: this breaks the connection both ways :(
                // this.closeVideo(user.username);
                // return;
            }

            // We need to broadcast video to connect to another.
            // TODO: because if the offerer doesn't add video tracks they
            // won't request video support so the answerer's video isn't sent
            if (!this.webcam.active) {
                this.ChatServer("You will need to turn your own camera on first before you can connect to " + user.username + ".");
                // return;
            }

            this.sendOpen(user.username);
        },
        closeVideo(username) {
            // A user has logged off the server. Clean up any WebRTC connections.
            delete (this.WebRTC.streams[username]);
            if (this.WebRTC.pc[username] != undefined) {
                this.WebRTC.pc[username].close();
                delete (this.WebRTC.pc[username]);
            }
        },

        // Stop broadcasting.
        stopVideo() {
            this.webcam.elem.srcObject = null;
            this.webcam.stream = null;
            this.webcam.active = false;

            // Close all WebRTC sessions.
            for (username of Object.keys(this.WebRTC.pc)) {
                this.closeVideo(username);
            }

            // Tell backend our camera state.
            this.sendMe();
        },

        initHistory(channel) {
            if (this.channels[channel] == undefined) {
                this.channels[channel] = {
                    history: [],
                    updated: Date.now(),
                    unread: 0,
                };
            }
        },
        pushHistory({ channel, username, message, action = "message", isChatServer, isChatClient }) {
            // Default channel = your current channel.
            if (!channel) {
                channel = this.channel;
            }

            // Initialize this channel's history?
            this.initHistory(channel);

            // Append the message.
            this.channels[channel].updated = Date.now();
            this.channels[channel].history.push({
                action: action,
                username: username,
                message: message,
                isChatServer,
                isChatClient,
            });
            this.scrollHistory();

            // Mark unread notifiers if this is not our channel.
            if (this.channel !== channel) {
                this.channels[channel].unread++;
            }
        },

        scrollHistory() {
            window.requestAnimationFrame(() => {
                this.historyScrollbox.scroll({
                    top: this.historyScrollbox.scrollHeight,
                    behavior: 'smooth',
                });
            });

        },

        // Responsive CSS controls for mobile. Notes for maintenance:
        // - The chat.css has responsive CSS to hide the left/right panels
        //   and set the grid-template-columns for devices < 1024px width
        // - These functions override w/ style tags to force the drawer to
        //   be visible and change the grid-template-columns.
        // - On window resize (e.g. device rotation) or when closing one
        //   of the side drawers, we reset our CSS overrides to default so
        //   the main chat window reappears.
        openChannelsPanel() {
            // Open the left drawer
            let $container = this.responsive.nodes.$container,
                $drawer = this.responsive.nodes.$left;

            $container.style.gridTemplateColumns = "1fr 0 0";
            $drawer.style.display = "block";
        },
        openWhoPanel() {
            // Open the right drawer
            let $container = this.responsive.nodes.$container,
                $drawer = this.responsive.nodes.$right;

            $container.style.gridTemplateColumns = "0 0 1fr";
            $drawer.style.display = "block";
        },
        openChatPanel() {
            this.resetResponsiveCSS();
        },
        resetResponsiveCSS() {
            let $container = this.responsive.nodes.$container,
                $left = this.responsive.nodes.$left,
                $right = this.responsive.nodes.$right;

            $left.style.removeProperty("display");
            $right.style.removeProperty("display");
            $container.style.removeProperty("grid-template-columns");
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