// console.log("BareRTC!");

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
            // busy: false, // TODO: not used
            disconnect: false,    // don't try to reconnect (e.g. kicked)
            windowFocused: true,  // browser tab is active
            windowFocusedAt: new Date(),

            // Disconnect spamming: don't retry too many times.
            disconnectLimit: 8,
            disconnectCount: 0,

            // Website configuration provided by chat.html template.
            config: {
                channels: PublicChannels,
                website: WebsiteURL,
                permitNSFW: PermitNSFW,
                sounds: {
                    available: SoundEffects,
                    settings: DefaultSounds,
                    ready: false,
                    audioContext: null,
                    audioTracks: {},
                }
            },

            // User JWT settings if available.
            jwt: {
                token: UserJWTToken,
                valid: UserJWTValid,
                claims: UserJWTClaims
            },

            channel: "lobby",
            username: "", //"test",
            autoLogin: false,  // e.g. from JWT auth
            message: "",
            typingNotifDebounce: null,

            // WebSocket connection.
            ws: {
                conn: null,
                connected: false,
            },

            // Who List for the room.
            whoList: [],
            whoMap: {}, // map username to wholist entry

            // My video feed.
            webcam: {
                busy: false,
                active: false,
                elem: null,   // <video id="localVideo"> element
                stream: null, // MediaStream object
                muted: false, // our outgoing mic is muted, not by default
                nsfw: false,  // user has flagged their camera to be NSFW

                // Who all is watching me? map of users.
                watching: {},

                // Scaling setting for the videos drawer, so the user can
                // embiggen the webcam sizes so a suitable size.
                videoScale: "",
                videoScaleOptions: [
                    [ "", "Default size" ],
                    [ "x1", "50% larger videos" ],
                    [ "x2", "2x larger videos" ],
                    [ "x3", "3x larger videos" ],
                    [ "x4", "4x larger videos (not recommended)" ],
                ],
            },

            // WebRTC sessions with other users.
            WebRTC: {
                // Streams per username.
                streams: {},
                muted: {}, // muted bool per username

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

            // Responsive CSS controls for mobile.
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

            settingsModal: {
                visible: false,
            },

            nsfwModalCast: {
                visible: false,
            },

            nsfwModalView: {
                visible: false,
                dontShowAgain: false,
                user: null, // staged User we wanted to open
            },
        }
    },
    mounted() {
        this.setupSounds();

        this.webcam.elem = document.querySelector("#localVideo");
        this.historyScrollbox = document.querySelector("#chatHistory");

        this.responsive.nodes = {
            $container: document.querySelector(".chat-container"),
            $left: document.querySelector(".left-column"),
            $center: document.querySelector(".chat-column"),
            $right: document.querySelector(".right-column"),
        };

        // Reset CSS overrides for responsive display on any window size change. In effect,
        // making the chat panel the current screen again on phone rotation.
        window.addEventListener("resize", () => {
            this.resetResponsiveCSS();
        });

        // Listen for window focus/unfocus events. Being on a different browser tab, for
        // sound effect alert purposes, counts as not being "in" that chat channel when
        // a message comes in.
        window.addEventListener("focus", () => {
            this.windowFocused = true;
            this.windowFocusedAt = new Date();
        });
        window.addEventListener("blur", () => {
            this.windowFocused = false;
        })

        for (let channel of this.config.channels) {
            this.initHistory(channel.ID);
        }

        this.ChatClient("Welcome to BareRTC!");

        // Auto login with JWT token?
        // TODO: JWT validation on the WebSocket as well.
        if (this.jwt.valid && this.jwt.claims.sub) {
            this.username = this.jwt.claims.sub;
            this.autoLogin = true;
        }

        // Scrub JWT token from query string parameters.
        history.pushState(null, "", location.href.split("?")[0]);

        // XX: always show login dialog to test if this helps iOS devices.
        this.loginModal.visible = true;
        /*
        if (!this.username) {
            this.loginModal.visible = true;
        } else {
            this.signIn();
        }
        */
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
        isDM() {
            // Is the current channel a DM?
            return this.channel.indexOf("@") === 0;
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

            // console.debug("Send message: %s", this.message);
            this.ws.conn.send(JSON.stringify({
                action: "message",
                channel: this.channel,
                message: this.message,
            }));

            this.message = "";
        },

        sendTypingNotification() {
            // TODO
        },

        // Sync the current user state (such as video broadcasting status) to
        // the backend, which will reload everybody's Who List.
        sendMe() {
            this.ws.conn.send(JSON.stringify({
                action: "me",
                videoActive: this.webcam.active,
                nsfw: this.webcam.nsfw,
            }));
        },
        onMe(msg) {
            // We have had settings pushed to us by the server, such as a change
            // in our choice of username.
            if (this.username != msg.username) {
                this.ChatServer(`Your username has been changed to ${msg.username}.`);
                this.username = msg.username;
            }

            // The server can set our webcam NSFW flag.
            if (this.webcam.nsfw != msg.nsfw) {
                this.webcam.nsfw = msg.nsfw;
            }

            // this.ChatClient(`User sync from backend: ${JSON.stringify(msg)}`);
        },

        // WhoList updates.
        onWho(msg) {
            this.whoList = msg.whoList;
            this.whoMap = {};

            // If we had a camera open with any of these and they have gone
            // off camera, close our side of the connection.
            for (let row of this.whoList) {
                this.whoMap[row.username] = row;
                if (this.WebRTC.streams[row.username] != undefined &&
                    row.videoActive !== true) {
                    this.closeVideo(row.username, "offerer");
                }
            }

            // Has the back-end server forgotten we are on video? This can
            // happen if we disconnect/reconnect while we were streaming.
            if (this.webcam.active && !this.whoMap[this.username]?.videoActive) {
                this.sendMe();
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
            // console.log("OPEN: connect to %s with secret %s", msg.username, secret);
            // this.ChatClient(`onOpen called for ${msg.username}.`);

            this.startWebRTC(msg.username, true);
        },
        onRing(msg) {
            // Message for the receiver to begin WebRTC connection.
            const secret = msg.openSecret;
            // console.log("RING: connection from %s with secret %s", msg.username, secret);
            this.ChatServer(`${msg.username} has opened your camera.`);

            this.startWebRTC(msg.username, false);
        },
        onUserExited(msg) {
            // A user has logged off the server. Clean up any WebRTC connections.
            this.closeVideo(msg.username);
        },

        // Handle messages sent in chat.
        onMessage(msg) {
            // Play sound effects if this is not the active channel or the window is not focused.
            if (msg.channel.indexOf("@") === 0) {
                if (msg.channel !== this.channel || !this.windowFocused) {
                    this.playSound("DM");
                }
            } else if (msg.channel !== this.channel || !this.windowFocused) {
                this.playSound("Chat");
            }

            this.pushHistory({
                channel: msg.channel,
                username: msg.username,
                message: msg.message,
            });
        },

        // User logged in or out.
        onPresence(msg) {
            // TODO: make a dedicated leave event
            if (msg.message.indexOf("has exited the room!") > -1) {
                // Clean up data about this user.
                this.onUserExited(msg);
                this.playSound("Leave");
            } else {
                this.playSound("Enter");
            }

            // Push it to the history of all public channels.
            for (let channel of this.config.channels) {
                this.pushHistory({
                    channel: channel.ID,
                    action: msg.action,
                    username: msg.username,
                    message: msg.message,
                });
            }

            // Push also to any DM channels for this user.
            let channel = "@" + msg.username;
            if (this.channels[channel] != undefined) {
                this.pushHistory({
                    channel: channel,
                    action: msg.action,
                    username: msg.username,
                    message: msg.message,
                });
            }
        },

        // Dial the WebSocket connection.
        dial() {
            this.ChatClient("Establishing connection to server...");

            const proto = location.protocol === 'https:' ? 'wss' : 'ws';
            const conn = new WebSocket(`${proto}://${location.host}/ws`);

            conn.addEventListener("close", ev => {
                // Lost connection to server - scrub who list.
                this.onWho({whoList: []});

                this.ws.connected = false;
                this.ChatClient(`WebSocket Disconnected code: ${ev.code}, reason: ${ev.reason}`);

                this.disconnectCount++;
                if (this.disconnectCount > this.disconnectLimit) {
                    this.ChatClient(`It seems there's a problem connecting to the server. Please try some other time. Note that iPhones and iPads currently have issues connecting to the chat room in general.`);
                    return;
                }

                if (!this.disconnect) {
                    if (ev.code !== 1001) {
                        this.ChatClient("Reconnecting in 5s");
                        setTimeout(this.dial, 5000);
                    }
                }
            });

            conn.addEventListener("open", ev => {
                this.ws.connected = true;
                this.ChatClient("Websocket connected!");

                // Tell the server our username.
                this.ws.conn.send(JSON.stringify({
                    action: "login",
                    username: this.username,
                    jwt: this.jwt.token,
                }));
            });

            conn.addEventListener("message", ev => {
                if (typeof ev.data !== "string") {
                    console.error("unexpected message type", typeof ev.data);
                    return;
                }

                let msg = JSON.parse(ev.data);
                try {
                    // Cast timestamp to date.
                    msg.at = new Date(msg.at);
                } catch(e) {
                    console.error("Parsing timestamp '%s' on msg: %s", msg.at, e);
                }

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
                    case "error":
                        this.pushHistory({
                            channel: msg.channel,
                            username: msg.username || 'Internal Server Error',
                            message: msg.message,
                            isChatServer: true,
                        });
                        break;
                    case "disconnect":
                        this.disconnect = true;
                        break;
                    case "ping":
                        break;
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

            // Store uni-directional PeerConnections:
            // - If we are reading video from the other (offerer)
            // - If we are sending video to the other (answerer)
            if (this.WebRTC.pc[username] == undefined) {
                this.WebRTC.pc[username] = {};
            }
            if (isOfferer) {
                this.WebRTC.pc[username].offerer = pc;
            } else {
                this.WebRTC.pc[username].answerer = pc;
            }

            // Keep a pointer to the current channel being established (for candidate/SDP).
            this.WebRTC.pc[username].connecting = pc;

            // Create a data channel so we have something to connect over even if
            // the local user is not broadcasting their own camera.
            let dataChannel = pc.createDataChannel("data");
            dataChannel.addEventListener("open", event => {
                // beginTransmission(dataChannel);
            })

            // 'onicecandidate' notifies us whenever an ICE agent needs to deliver a
            // message to the other peer through the signaling server.
            pc.onicecandidate = event => {
                if (event.candidate) {
                    this.ws.conn.send(JSON.stringify({
                        action: "candidate",
                        username: username,
                        candidate: JSON.stringify(event.candidate),
                    }));
                }
            };

            // If the user is offerer let the 'negotiationneeded' event create the offer.
            if (isOfferer) {
                pc.onnegotiationneeded = () => {
                    pc.createOffer().then(this.localDescCreated(pc, username)).catch(this.ChatClient);
                };
            }

            // When a remote stream arrives.
            pc.ontrack = event => {
                const stream = event.streams[0];

                // Do we already have it?
                // this.ChatClient(`Received a video stream from ${username}.`);
                if (this.WebRTC.streams[username] == undefined ||
                    this.WebRTC.streams[username].id !== stream.id) {
                    this.WebRTC.streams[username] = stream;
                }

                window.requestAnimationFrame(() => {
                    let $ref = document.getElementById(`videofeed-${username}`);
                    $ref.srcObject = stream;
                });

                // Inform them they are being watched.
                this.sendWatch(username, true);
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
                            description: JSON.stringify(pc.localDescription),
                        }));
                    },
                    console.error,
                )
            };
        },

        // Handle inbound WebRTC signaling messages proxied by the websocket.
        onCandidate(msg) {
            if (this.WebRTC.pc[msg.username] == undefined || !this.WebRTC.pc[msg.username].connecting) {
                return;
            }
            let pc = this.WebRTC.pc[msg.username].connecting;

            // XX: WebRTC candidate/SDP messages JSON stringify their inner payload so that the
            // Go back-end server won't re-order their json keys (Safari on Mac OS is very sensitive
            // to the keys being re-ordered during the handshake, in ways that NO OTHER BROWSER cares
            // about at all). Re-parse the JSON stringified object here.
            let candidate = JSON.parse(msg.candidate);

            // Add the new ICE candidate.
            pc.addIceCandidate(
                new RTCIceCandidate(
                    candidate,
                    () => { },
                    console.error,
                )
            );
        },
        onSDP(msg) {
            if (this.WebRTC.pc[msg.username] == undefined || !this.WebRTC.pc[msg.username].connecting) {
                return;
            }
            let pc = this.WebRTC.pc[msg.username].connecting;

            // XX: WebRTC candidate/SDP messages JSON stringify their inner payload so that the
            // Go back-end server won't re-order their json keys (Safari on Mac OS is very sensitive
            // to the keys being re-ordered during the handshake, in ways that NO OTHER BROWSER cares
            // about at all). Re-parse the JSON stringified object here.
            let message = JSON.parse(msg.description);

            // Add the new ICE candidate.
            // this.ChatClient(`Received a Remote Description from ${msg.username}: ${JSON.stringify(msg.description)}.`);
            pc.setRemoteDescription(new RTCSessionDescription(message), () => {
                // When receiving an offer let's answer it.
                if (pc.remoteDescription.type === 'offer') {
                    pc.createAnswer().then(this.localDescCreated(pc, msg.username)).catch(this.ChatClient);
                }
            }, console.error);
        },
        onWatch(msg) {
            // The user has our video feed open now.
            this.webcam.watching[msg.username] = true;
        },
        onUnwatch(msg) {
            // The user has closed our video feed.
            delete(this.webcam.watching[msg.username]);
        },
        sendWatch(username, watching) {
            // Send the watch or unwatch message to backend.
            this.ws.conn.send(JSON.stringify({
                action: watching ? "watch" : "unwatch",
                username: username,
            }));
        },

        /**
         * Front-end web app concerns.
         */

        // Settings modal.
        showSettings() {
            this.settingsModal.visible = true;
        },
        hideSettings() {
            this.settingsModal.visible = false;
        },

        // Set active chat room.
        setChannel(channel) {
            this.channel = typeof(channel) === "string" ? channel : channel.ID;
            this.scrollHistory();
            this.channels[this.channel].unread = 0;

            // Responsive CSS: switch back to chat panel upon selecting a channel.
            this.openChatPanel();

            // Edit hyperlinks to open in a new window.
            this.makeLinksExternal();
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

            // Responsive CSS: switch back to chat panel upon opening a DM.
            this.openChatPanel();
        },
        openProfile(user) {
            let url = this.profileURLForUsername(user.username);
            if (url) {
                window.open(url);
            }
        },
        avatarURL(user) {
            // Resolve the avatar URL of this user.
            if (user.avatar.match(/^https?:/i)) {
                return user.avatar;
            } else if (user.avatar.indexOf("/") === 0) {
                return this.config.website.replace(/\/+$/, "") + user.avatar;
            }
            return "";
        },
        avatarForUsername(username) {
            if (this.whoMap[username] != undefined && this.whoMap[username].avatar) {
                return this.avatarURL(this.whoMap[username]);
            }
            return null;
        },
        profileURLForUsername(username) {
            if (!username) return;
            username = username.replace(/^@/, "");
            if (this.whoMap[username] != undefined && this.whoMap[username].profileURL) {
                let url = this.whoMap[username].profileURL;
                if (url.match(/^https?:/i)) {
                    return url;
                } else if (url.indexOf("/") === 0) {
                    // Subdirectory relative to our WebsiteURL
                    return this.config.website.replace(/\/+$/, "") + url;
                } else {
                    this.ChatClient("Didn't know how to open profile URL: " + url);
                }
                return url;
            }
            return null;
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
        startVideo(force) {
            if (this.webcam.busy) return;

            // If we are running in PermitNSFW mode, show the user the modal.
            if (this.config.permitNSFW && !force) {
                this.nsfwModalCast.visible = true;
                return;
            }

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
        openVideo(user, force) {
            if (user.username === this.username) {
                this.ChatClient("You can already see your own webcam.");
                return;
            }

            // Is the target user NSFW? Go thru the modal.
            let dontShowAgain = localStorage["skip-nsfw-modal"] == "true";
            if (user.nsfw && !dontShowAgain && !force) {
                this.nsfwModalView.user = user;
                this.nsfwModalView.visible = true;
                return;
            }
            if (this.nsfwModalView.dontShowAgain) {
                // user doesn't want to see the modal again.
                localStorage["skip-nsfw-modal"] = "true";
            }

            // Camera is already open? Then disconnect the connection.
            if (this.WebRTC.pc[user.username] != undefined && this.WebRTC.pc[user.username].offerer != undefined) {
                this.closeVideo(user.username, "offerer");
                return;
            }

            this.sendOpen(user.username);

            // Responsive CSS -> go to chat panel to see the camera
            this.openChatPanel();
        },
        closeVideo(username, name) {
            if (name === "offerer") {
                // We are closing another user's video stream.
                delete (this.WebRTC.streams[username]);
                if (this.WebRTC.pc[username] != undefined && this.WebRTC.pc[username].offerer != undefined) {
                    this.WebRTC.pc[username].offerer.close();
                    delete (this.WebRTC.pc[username]);
                }

                // Inform backend we have closed it.
                this.sendWatch(username, false);
                return;
            } else if (name === "answerer") {
                // We have turned off our camera, kick off viewers.
                if (this.WebRTC.pc[username] != undefined && this.WebRTC.pc[username].answerer != undefined) {
                    this.WebRTC.pc[username].answerer.close();
                    delete (this.WebRTC.pc[username]);
                }
                return;
            }

            // A user has logged off the server. Clean up any WebRTC connections.
            delete (this.WebRTC.streams[username]);
            delete (this.webcam.watching[username]);
            if (this.WebRTC.pc[username] != undefined) {
                if (this.WebRTC.pc[username].offerer) {
                    this.WebRTC.pc[username].offerer.close();
                }
                if (this.WebRTC.pc[username].answerer) {
                    this.WebRTC.pc[username].answerer.close();
                }
                delete (this.WebRTC.pc[username]);
            }

            // Inform backend we have closed it.
            this.sendWatch(username, false);
        },

        // Show who watches our video.
        showViewers() {
            // TODO: for now, ChatClient is our bro.
            let users = Object.keys(this.webcam.watching);
            if (users.length === 0) {
                this.ChatClient("There is currently nobody viewing your camera.");
            } else {
                this.ChatClient("Your current webcam viewers are:<br><br>" + users.join(", "));
            }
        },

        // Stop broadcasting.
        stopVideo() {
            this.webcam.elem.srcObject = null;
            this.webcam.stream = null;
            this.webcam.active = false;

            // Close all WebRTC sessions.
            for (username of Object.keys(this.WebRTC.pc)) {
                this.closeVideo(username, "answerer");
            }

            // Tell backend our camera state.
            this.sendMe();
        },

        // Mute my microphone if broadcasting.
        muteMe() {
            this.webcam.muted = !this.webcam.muted;
            this.webcam.stream.getAudioTracks().forEach(track => {
                track.enabled = !this.webcam.muted;
            });
        },
        isMuted(username) {
            return this.WebRTC.muted[username] === true;
        },
        muteVideo(username) {
            this.WebRTC.muted[username] = !this.isMuted(username);

            // Find the <video> tag to mute it.
            let $ref = document.getElementById(`videofeed-${username}`);
            if ($ref) {
                $ref.muted = this.WebRTC.muted[username];
            }
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
                at: new Date(),
                isChatServer,
                isChatClient,
            });
            this.scrollHistory();

            // Mark unread notifiers if this is not our channel.
            if (this.channel !== channel) {
                // Don't notify about presence broadcasts.
                if (action !== "presence") {
                    this.channels[channel].unread++;
                }
            }

            // Edit hyperlinks to open in a new window.
            this.makeLinksExternal();
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

        // Format a datetime nicely for chat timestamp.
        prettyDate(date) {
            let hours = date.getHours(),
                minutes = String(date.getMinutes()).padStart(2, '0'),
                seconds = String(date.getSeconds()).padStart(2, '0'),
                ampm = hours >= 11 ? "pm" : "am";

            let hour = hours%12 || 12;
            return `${(hour)}:${minutes}:${seconds} ${ampm}`;
        },

        /**
         * Sound effect concerns.
         */

        setupSounds() {
            try {
                if (AudioContext) {
                    this.config.sounds.audioContext = new AudioContext();
                } else {
                    this.config.sounds.audioContext = window.AudioContext || window.webkitAudioContext;
                }
            } catch {}

            if (!this.config.sounds.audioContext) {
                console.error("Couldn't set up AudioContext! No sound effects will be supported.");
                return;
            }

            // Create <audio> elements for all the sounds.
            for (let effect of this.config.sounds.available) {
                if (!effect.filename) continue; // 'Quiet' has no audio

                let elem = document.createElement("audio");
                elem.autoplay = false;
                elem.src = `/static/sfx/${effect.filename}`;
                document.body.appendChild(elem);

                let track = this.config.sounds.audioContext.createMediaElementSource(elem);
                track.connect(this.config.sounds.audioContext.destination);
                this.config.sounds.audioTracks[effect.name] = elem;
            }

            // Apply the user's saved preferences if any.
            for (let setting of Object.keys(this.config.sounds.settings)) {
                if (localStorage[`sound:${setting}`] != undefined) {
                    this.config.sounds.settings[setting] = localStorage[`sound:${setting}`];
                }
            }
        },

        playSound(event) {
            let filename = this.config.sounds.settings[event];
            // Do we have an audio track?
            if (this.config.sounds.audioTracks[filename] != undefined) {
                let track = this.config.sounds.audioTracks[filename];
                track.play();
            }
        },

        setSoundPref(event) {
            this.playSound(event);

            // Store the user's setting in localStorage.
            localStorage[`sound:${event}`] = this.config.sounds.settings[event];
        },

        makeLinksExternal() {
            window.requestAnimationFrame(() => {
                let $history = document.querySelector("#chatHistory");
                // Make all <a> links appearing in chat into external links.
                ($history.querySelectorAll("a") || []).forEach(node => {
                    let href = node.attributes.href,
                        target = node.attributes.target;
                    if (href == undefined || target != undefined) return;
                    node.target = "_blank";
                });
            })
        },
    }
});

app.mount("#BareRTC-App");