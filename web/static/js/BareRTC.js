// console.log("BareRTC!");

// WebRTC configuration.
const configuration = {
    iceServers: TURN.URLs.map(val => {
        let row = {
            urls: val,
        };

        if (val.indexOf('turn:') === 0) {
            row.username = TURN.Username;
            row.credential = TURN.Credential;
        }

        return row;
    })
};

const FileUploadMaxSize = 1024 * 1024 * 8; // 8 MB


function setModalImage(url) {
    let $modalImg = document.querySelector("#modalImage"),
        $modal = document.querySelector("#photo-modal");
    $modalImg.src = url;
    $modal.classList.add("is-active");
    return false;
}

// Popped-out video drag functions.



const app = Vue.createApp({
    delimiters: ['[[', ']]'],
    data() {
        return {
            // busy: false, // TODO: not used
            disconnect: false,    // don't try to reconnect (e.g. kicked)
            windowFocused: true,  // browser tab is active
            windowFocusedAt: new Date(),

            // Disconnect spamming: don't retry too many times.
            disconnectLimit: 3,
            disconnectCount: 0,

            // Website configuration provided by chat.html template.
            config: {
                channels: PublicChannels,
                website: WebsiteURL,
                permitNSFW: PermitNSFW,
                fontSizeClasses: [
                    [ "", "Default size" ],
                    [ "x1", "50% larger chat room text" ],
                    [ "x2", "2x larger chat room text" ],
                    [ "x3", "3x larger chat room text" ],
                    [ "x4", "4x larger chat room text" ],
                ],
                sounds: {
                    available: SoundEffects,
                    settings: DefaultSounds,
                    ready: false,
                    audioContext: null,
                    audioTracks: {},
                },
                reactions: [
                    ['❤️', '👍', '😂', '😉', '😢', '😡', '🥰'],
                    ['👋', '🔥', '😈', '🍑', '🍆', '💦', '🍌'],
                    ['😋', '⭐', '😇', '😴', '😱', '👀', '🎃'],
                    ['😏', '🙈', '🙉', '🙊', '☀️', '🌈', '🎂']
                ]
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
            status: "online", // away/idle status

            // Idle detection variables
            idleTimeout: null,
            idleThreshold: 60, // number of seconds you must be idle

            // WebSocket connection.
            ws: {
                conn: null,
                connected: false,
            },

            // Who List for the room.
            whoList: [],
            whoTab: 'online',
            whoMap: {}, // map username to wholist entry
            muted: {},  // muted usernames for client side state

            // My video feed.
            webcam: {
                busy: false,
                active: false,
                elem: null,   // <video id="localVideo"> element
                stream: null, // MediaStream object
                muted: false, // our outgoing mic is muted, not by default
                nsfw: false,  // user has flagged their camera to be NSFW
                mutual: false, // user wants viewers to share their own videos
                mutualOpen: false, // user wants to open video mutually

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

                // Available cameras and microphones for the Settings modal.
                videoDevices: [],
                videoDeviceID: null,
                audioDevices: [],
                audioDeviceID: null,
            },

            // Video flag constants (sync with values in messages.go)
            VideoFlag: {
                Active:         1 << 0,
                NSFW:           1 << 1,
                Muted:          1 << 2,
                IsTalking:      1 << 3,
                MutualRequired: 1 << 4,
                MutualOpen:     1 << 5,
            },

            // WebRTC sessions with other users.
            WebRTC: {
                // Streams per username.
                streams: {},
                muted: {}, // muted bool per username
                poppedOut: {}, // popped-out video per username

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
            autoscroll: true, // scroll to bottom on new messages
            fontSizeClass: "", // font size magnification
            DMs: {},
            messageReactions: {
                // Will look like:
                // "123": {    (message ID)
                //    "❤️": [  (reaction emoji)
                //        "username"  // users who reacted
                //    ]
                // }
            },

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
        this.setupConfig(); // localSettings persisted settings
        this.setupIdleDetection();

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
        });

        // Set up sound effects on first page interaction.
        window.addEventListener("click", () => {
            this.setupSounds();
        });
        window.addEventListener("keydown", () => {
            this.setupSounds();
        });

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
        // this.loginModal.visible = true;
        if (!this.username) {
            this.loginModal.visible = true;
        } else {
            this.signIn();
        }
    },
    watch: {
        "webcam.videoScale": () => {
            document.querySelectorAll(".video-feeds > .feed").forEach(node => {
                node.style.width = null;
                node.style.height = null;
            });
        },
        fontSizeClass() {
            // Store the setting persistently.
            localStorage.fontSizeClass = this.fontSizeClass;
        },
        status() {
            // Send presence updates to the server.
            this.sendMe();
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
        isDM() {
            // Is the current channel a DM?
            return this.channel.indexOf("@") === 0;
        },
        isOp() {
            // Returns if the current user has operator rights
            return this.jwt.claims.op;
        },
        myVideoFlag() {
            // Compute the current user's video status flags.
            let status = 0;
            if (this.webcam.active) status |= this.VideoFlag.Active;
            if (this.webcam.muted) status |= this.VideoFlag.Muted;
            if (this.webcam.nsfw) status |= this.VideoFlag.NSFW;
            if (this.webcam.mutual) status |= this.VideoFlag.MutualRequired;
            if (this.webcam.mutualOpen) status |= this.VideoFlag.MutualOpen;
            return status;
        },
    },
    methods: {
        // Load user prefs from localStorage, called on startup
        setupConfig() {
            if (localStorage.fontSizeClass != undefined) {
                this.fontSizeClass = localStorage.fontSizeClass;
            }

            // Webcam mutality preferences from last broadcast.
            if (localStorage.videoMutual === "true") {
                this.webcam.mutual = true;
            }
            if (localStorage.videoMutualOpen === "true") {
                this.webcam.mutualOpen = true;
            }
        },

        signIn() {
            this.loginModal.visible = false;
            this.dial();
        },

        // Normalize a DM channel name into a username (remove the @ prefix)
        normalizeUsername(channel) {
            return channel.replace(/^@+/, '');
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

        // Emoji reactions
        sendReact(message, emoji) {
            this.ws.conn.send(JSON.stringify({
                action: 'react',
                msgID: message.msgID,
                message: emoji,
            }));
        },
        onReact(msg) {
            // Search all channels for this message ID and append the reaction.
            let msgID = msg.msgID,
                who = msg.username,
                emoji = msg.message;

            if (this.messageReactions[msgID] == undefined) {
                this.messageReactions[msgID] = {};
            }
            if (this.messageReactions[msgID][emoji] == undefined) {
                this.messageReactions[msgID][emoji] = [];
            }

            // if they double sent the same reaction, it counts as a removal
            let unreact = false;
            for (let i = 0; i < this.messageReactions[msgID][emoji].length; i++) {
                let reactor = this.messageReactions[msgID][emoji][i];
                if (reactor === who) {
                    this.messageReactions[msgID][emoji].splice(i, 1);
                    unreact = true;
                }
            }

            // if this emoji reaction is empty, clean it up
            if (unreact) {
                if (this.messageReactions[msgID][emoji].length === 0) {
                    delete(this.messageReactions[msgID][emoji]);
                }
                return;
            }

            this.messageReactions[msgID][emoji].push(who);
        },

        // Sync the current user state (such as video broadcasting status) to
        // the backend, which will reload everybody's Who List.
        sendMe() {
            this.ws.conn.send(JSON.stringify({
                action: "me",
                video: this.myVideoFlag,
                status: this.status,
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
            let myNSFW = this.webcam.nsfw;
            let theirNSFW = (msg.video & this.VideoFlag.NSFW) > 0;
            if (myNSFW != theirNSFW) {
                this.webcam.nsfw = theirNSFW;
            }
        },

        // WhoList updates.
        onWho(msg) {
            this.whoList = msg.whoList;
            this.whoMap = {};

            if (this.whoList == undefined) {
                this.whoList = [];
            }

            // If we had a camera open with any of these and they have gone
            // off camera, close our side of the connection.
            for (let row of this.whoList) {
                this.whoMap[row.username] = row;
                if (this.WebRTC.streams[row.username] != undefined &&
                    !(row.video & this.VideoFlag.Active)) {
                    this.closeVideo(row.username, "offerer");
                }
            }

            // Has the back-end server forgotten we are on video? This can
            // happen if we disconnect/reconnect while we were streaming.
            if (this.webcam.active && !(this.whoMap[this.username]?.video & this.VideoFlag.Active)) {
                this.sendMe();
            }
        },

        // Mute or unmute a user.
        muteUser(username) {
            username = this.normalizeUsername(username);
            let mute = this.muted[username] == undefined;
            if (mute) {
                if (!window.confirm(
                    `Do you want to mute ${username}? If muted, you will no longer see their `+
                    `chat messages or any DMs they send you going forward. Also, ${username} will `+
                    `not be able to see whether your webcam is active until you unmute them.`
                )) {
                    return;
                }
                this.muted[username] = true;
            } else {
                delete this.muted[username];
            }

            // Hang up videos both ways.
            this.closeVideo(username);

            this.sendMute(username, mute);
            if (mute) {
                this.ChatClient(
                    `You have muted <strong>${username}</strong> and will no longer see their chat messages, `+
                    `and they will not see whether your webcam is active. You may unmute them via the Who Is Online list.`);
            } else {
                this.ChatClient(
                    `You have unmuted <strong>${username}</strong> and can see their chat messages from now on.`,
                );
            }
        },
        sendMute(username, mute) {
            this.ws.conn.send(JSON.stringify({
                action: mute ? "mute" : "unmute",
                username: username,
            }));
        },
        isMutedUser(username) {
            return this.muted[this.normalizeUsername(username)] != undefined;
        },

        // Send a video request to access a user's camera.
        sendOpen(username) {
            this.ws.conn.send(JSON.stringify({
                action: "open",
                username: username,
            }));
        },
        sendBoot(username) {
            this.ws.conn.send(JSON.stringify({
                action: "boot",
                username: username,
            }));
        },
        onOpen(msg) {
            // Response for the opener to begin WebRTC connection.
            this.startWebRTC(msg.username, true);
        },
        onRing(msg) {
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
                messageID: msg.msgID,
            });
        },

        // A user deletes their message for everybody
        onTakeback(msg) {
            // Search all channels for this message ID and remove it.
            for (let channel of Object.keys(this.channels)) {
                for (let i = 0; i < this.channels[channel].history.length; i++) {
                    let cmp = this.channels[channel].history[i];
                    if (cmp.msgID === msg.msgID) {
                        this.channels[channel].history.splice(i, 1);
                        return;
                    }
                }
            }

            console.error("Got a takeback for msgID %d but did not find it!", msg.msgID);
        },

        // User logged in or out.
        onPresence(msg) {
            // TODO: make a dedicated leave event
            let isLeave = false;
            if (msg.message.indexOf("has exited the room!") > -1) {
                // Clean up data about this user.
                this.onUserExited(msg);
                this.playSound("Leave");
                isLeave = true;
            } else {
                this.playSound("Enter");
            }

            // Push it to the history of all public channels (not leaves).
            if (!isLeave) {
                for (let channel of this.config.channels) {
                    this.pushHistory({
                        channel: channel.ID,
                        action: msg.action,
                        username: msg.username,
                        message: msg.message,
                    });
                }
            }

            // Push also to any DM channels for this user (leave events do push to DM thread for visibility).
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
                this.muted = {};

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
                        // New JWT token?
                        if (msg.jwt) {
                            this.jwt.token = msg.jwt;
                        }

                        // Reset disconnect retry counter: if we were on long enough to get
                        // a ping, we're well connected and can reconnect no matter how many
                        // times the chat server is rebooted.
                        this.disconnectCount = 0;
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

            // If we are the offerer, and this member wants to auto-open our camera
            // then add our own stream to the connection.
            if (isOfferer && (this.whoMap[username].video & this.VideoFlag.MutualOpen) && this.webcam.active) {
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
            this.scrollHistory(this.channel, true);
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
        isUsernameOnline(username) {
            return this.whoMap[username] != undefined;
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
        nicknameForUsername(username) {
            if (!username) return;
            username = username.replace(/^@/, "");
            if (this.whoMap[username] != undefined && this.whoMap[username].profileURL) {
                let nick = this.whoMap[username].nickname;
                if (nick) {
                    return nick;
                }
            } else if (this.whoMap[username] == undefined && username !== 'ChatServer' && username !== 'ChatClient') {
                // User is not even logged in! Add this note to their name
                username += " (offline)";
            }
            return username;
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

        /* Take back messages (for everyone) or remove locally */
        takeback(msg) {
            if (!window.confirm(
                "Do you want to take this message back? Doing so will remove this message from everybody's view in the chat room."
            )) return;

            this.ws.conn.send(JSON.stringify({
                action: "takeback",
                msgID: msg.msgID,
            }));
        },
        removeMessage(msg) {
            if (!window.confirm(
                "Do you want to remove this message from your view? This will delete the message only for you, but others in this chat thread may still see it."
            )) return;

            this.onTakeback({
                msgID: msg.msgID,
            })
        },

        /* message reaction emojis */
        hasReactions(msg) {
            return this.messageReactions[msg.msgID] != undefined;
        },
        getReactions(msg) {
            if (!this.hasReactions(msg)) return [];
            return this.messageReactions[msg.msgID];
        },
        iReacted(msg, emoji) {
            // test whether the current user has reacted
            if (this.messageReactions[msg.msgID] != undefined && this.messageReactions[msg.msgID][emoji] != undefined) {
                for (let reactor of this.messageReactions[msg.msgID][emoji]) {
                    if (reactor === this.username) {
                        return true;
                    }
                }
            }
            return false;
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

                // Save our mutuality prefs.
                localStorage.videoMutual = this.webcam.mutual;
                localStorage.videoMutualOpen = this.webcam.mutualOpen;

                // Tell backend the camera is ready.
                this.sendMe();

                // Record the selected device IDs.
                this.webcam.videoDeviceID = stream.getVideoTracks()[0].getSettings().deviceId;
                this.webcam.audioDeviceID = stream.getAudioTracks()[0].getSettings().deviceId;
                console.log("device IDs:", this.webcam.videoDeviceID, this.webcam.audioDeviceID);

                // Collect video and audio devices to let the user change them in their settings.
                this.getDevices();
            }).catch(err => {
                this.ChatClient(`Webcam error: ${err}`);
            }).finally(() => {
                this.webcam.busy = false;
            })
        },
        getDevices() {
            // Collect video and audio devices.
            if (!navigator.mediaDevices?.enumerateDevices) {
                console.log("enumerateDevices() not supported.");
                return;
            }

            navigator.mediaDevices.enumerateDevices().then(devices => {
                this.webcam.videoDevices = [];
                this.webcam.audioDevices = [];
                devices.forEach(device => {
                    if (device.kind === 'videoinput') {
                        this.webcam.videoDevices.push({
                            id: device.deviceId,
                            label: device.label,
                        });
                    } else if (device.kind === 'audioinput') {
                        this.webcam.audioDevices.push({
                            id: device.deviceId,
                            label: device.label,
                        });
                    }
                })
            }).catch(err => {
                this.ChatClient(`Error listing your cameras and microphones: ${err.name}: ${err.message}`);
            })
        },

        // Begin connecting to someone else's webcam.
        openVideo(user, force) {
            if (user.username === this.username) {
                this.ChatClient("You can already see your own webcam.");
                return;
            }

            // If we have muted the target, we shouldn't view their video.
            if (this.isMutedUser(user.username)) {
                this.ChatClient(`You have muted <strong>${user.username}</strong> and so should not see their camera.`);
                return;
            }

            // Is the target user NSFW? Go thru the modal.
            let dontShowAgain = localStorage["skip-nsfw-modal"] == "true";
            if ((user.video & this.VideoFlag.NSFW) && !dontShowAgain && !force) {
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

            // If this user requests mutual viewership...
            if ((user.video & this.VideoFlag.MutualRequired) && !this.webcam.active) {
                this.ChatClient(
                    `<strong>${user.username}</strong> has requested that you should share your own camera too before opening theirs.`
                );
                return;
            }

            this.sendOpen(user.username);

            // Responsive CSS -> go to chat panel to see the camera
            this.openChatPanel();

            // Send some feedback to the chat window.
            this.ChatClient(
                `A request was sent to open <strong>${user.username}</strong>'s camera which should (hopefully) appear on your screen soon.<br><br>`+
                `<strong class="has-text-danger">Notice:</strong> webcam sharing currently does not work well with iPhones, iPads or Safari browsers. It should generally `+
                `work well on Firefox or Chrome-like browsers on <em>most</em> devices (including Macbooks) but at this time there is no working `+
                `option for iPhone/iPad. (Chrome-like browsers also include Edge, Brave, or Opera). If their video does not open or you get a blank `+
                `screen, try logging on from a different web browser.`
            );
        },
        closeVideo(username, name) {
            if (name === "offerer") {
                // We are closing another user's video stream.
                delete (this.WebRTC.streams[username]);
                delete (this.WebRTC.muted[username]);
                delete (this.WebRTC.poppedOut[username]);
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
                delete (this.WebRTC.muted[username]);
                delete (this.WebRTC.poppedOut[username]);
            }

            // Inform backend we have closed it.
            this.sendWatch(username, false);
        },
        unMutualVideo() {
            // If we had our camera on to watch a video of someone who wants mutual cameras,
            // and then we turn ours off: we should unfollow the ones with mutual video.
            for (let row of this.whoList) {
                let username = row.username;
                if ((row.video & this.VideoFlag.MutualRequired) && this.WebRTC.pc[username] != undefined) {
                    this.closeVideo(username);
                }
            }
        },
        webcamIconClass(user) {
            // Return the icon to show on a video button.
            // - Usually a video icon
            // - May be a crossed-out video if isVideoNotAllowed
            // - Or an eyeball for cameras already opened
            if (user.username === this.username && this.webcam.active) {
                return 'fa-eye'; // user sees their own self camera always
            }

            // Already opened?
            if (this.WebRTC.pc[user.username] != undefined && this.WebRTC.streams[user.username] != undefined) {
                return 'fa-eye';
            }

            if (this.isVideoNotAllowed(user)) return 'fa-video-slash';
            return 'fa-video';
        },
        isVideoNotAllowed(user) {
            // Returns whether the video button to open a user's cam will be not allowed (crossed out)

            // Mutual video sharing is required on this camera, and ours is not active
            if ((user.video & this.VideoFlag.Active) && (user.video & this.VideoFlag.MutualRequired) && !this.webcam.active) {
                return true;
            }

            // We have muted them and it wouldn't be appropriate to still watch their video but not get their messages.
            if (this.isMutedUser(user.username)) {
                return true;
            }

            return false;
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

            // Also focus the Watching list.
            this.whoTab = 'watching';

            // TODO: if mobile, show the panel - this width matches
            // the media query in chat.css
            if (screen.width < 1024) {
                this.openWhoPanel();
            }
        },

        // Boot someone off yourn video.
        bootUser(username) {
            if (!window.confirm(
                `Kick ${username} off your camera? This will also prevent them `+
                `from seeing that your camera is active for the remainder of your `+
                `chat session.`)) {
                return;
            }

            this.sendBoot(username);

            // Close the WebRTC peer connection.
            if (this.WebRTC.pc[username] != undefined) {
                this.closeVideo(username, "answerer");
            }

            this.ChatClient(
                `You have booted ${username} off your camera. They will no longer be able `+
                `to connect to your camera, or even see that your camera is active at all -- `+
                `to them it appears as though you had turned yours off.<br><br>This will be `+
                `in place for the remainder of your current chat session.`
            );
        },

        // Stop broadcasting.
        stopVideo() {
            // Close all WebRTC sessions.
            for (username of Object.keys(this.WebRTC.pc)) {
                this.closeVideo(username, "answerer");
            }

            // Hang up on mutual cameras.
            this.unMutualVideo();

            // Close the local camera devices completely.
            this.webcam.stream.getTracks().forEach(track => {
                track.stop();
            });

            // Reset all front-end state.
            this.webcam.elem.srcObject = null;
            this.webcam.stream = null;
            this.webcam.active = false;
            this.webcam.muted = false;
            this.whoTab = "online";

            // Tell backend our camera state.
            this.sendMe();
        },

        // Mute my microphone if broadcasting.
        muteMe() {
            this.webcam.muted = !this.webcam.muted;
            this.webcam.stream.getAudioTracks().forEach(track => {
                track.enabled = !this.webcam.muted;
            });

            // Communicate our local mute to others.
            this.sendMe();
        },
        isSourceMuted(username) {
            // See if the webcam broadcaster muted their mic at the source
            if (this.whoMap[username] != undefined && this.whoMap[username].video & this.VideoFlag.Muted) {
                return true;
            }
            return false;
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

        // Pop out a user's video.
        popoutVideo(username) {
            this.WebRTC.poppedOut[username] = !this.WebRTC.poppedOut[username];

            // If not popped out, reset CSS positioning.
            window.requestAnimationFrame(this.makeDraggableVideos);
        },

        // Outside of Vue, attach draggable video scripts to DOM.
        makeDraggableVideos() {
            let $panel = document.querySelector("#video-feeds");

            interact('.popped-in').unset();

            // Give popped out videos to the root of the DOM so they can
            // be dragged anywhere on the page.
            window.requestAnimationFrame(() => {
                document.querySelectorAll('.popped-out').forEach(node => {
                    // $panel.removeChild(node);
                    document.body.appendChild(node);
                });

                document.querySelectorAll('.popped-in').forEach(node => {
                    // document.body.removeChild(node);
                    $panel.appendChild(node);
                    node.style.top = null;
                    node.style.left = null;
                    node.setAttribute('data-x', 0);
                    node.setAttribute('data-y', 0);
                });
            });

            interact('.popped-out').draggable({
                // enable inertial throwing
                inertia: true,
                // keep the element within the area of it's parent
                modifiers: [
                interact.modifiers.restrictRect({
                    restriction: 'parent',
                    endOnly: true
                })
                ],

                listeners: {
                // call this function on every dragmove event
                move(event) {
                    let target = event.target;
                    let x = (parseFloat(target.getAttribute('data-x')) || 0) + event.dx
                    let y = (parseFloat(target.getAttribute('data-y')) || 0) + event.dy

                    target.style.top = `${y}px`;
                    target.style.left = `${x}px`;

                    target.setAttribute('data-x', x);
                    target.setAttribute('data-y', y);
                },

                // call this function on every dragend event
                end (event) {
                    console.log(
                    'moved a distance of ' +
                    (Math.sqrt(Math.pow(event.pageX - event.x0, 2) +
                                Math.pow(event.pageY - event.y0, 2) | 0))
                        .toFixed(2) + 'px')
                }
                }
            }).resizable({
                edges: { left: true, right: true, bottom: true, right: true },
                listeners: {
                    move (event) {
                      var target = event.target
                      var x = (parseFloat(target.getAttribute('data-x')) || 0)
                      var y = (parseFloat(target.getAttribute('data-y')) || 0)

                      // update the element's style
                      target.style.width = event.rect.width + 'px'
                      target.style.height = event.rect.height + 'px'

                      // translate when resizing from top or left edges
                      x += event.deltaRect.left
                      y += event.deltaRect.top

                      target.style.top = `${y}px`;
                      target.style.left = `${x}px`;

                      target.setAttribute('data-x', x)
                      target.setAttribute('data-y', y)
                    }
                  },
                  modifiers: [
                    // keep the edges inside the parent
                    interact.modifiers.restrictEdges({
                      outer: 'parent'
                    }),

                    // minimum size
                    interact.modifiers.restrictSize({
                      min: { width: 100, height: 50 }
                    })
                  ],

                  inertia: true
            })
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
        pushHistory({ channel, username, message, action = "message", isChatServer, isChatClient, messageID }) {
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
                msgID: messageID,
                at: new Date(),
                isChatServer,
                isChatClient,
            });
            this.scrollHistory(channel);

            // Mark unread notifiers if this is not our channel.
            if (this.channel !== channel) {
                // Don't notify about presence broadcasts.
                if (action !== "presence" && !isChatServer) {
                    this.channels[channel].unread++;
                }
            }

            // Edit hyperlinks to open in a new window.
            this.makeLinksExternal();
        },

        scrollHistory(channel, force) {
            if (!this.autoscroll && !force) return;

            window.requestAnimationFrame(() => {
                // Only scroll if it's the current channel.
                if (channel !== this.channel) return;

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
                ampm = hours >= 11 ? "pm" : "am";

            let hour = hours%12 || 12;
            return `${(hour)}:${minutes} ${ampm}`;
        },

        /**
         * Image sharing in chat
         */

        // The image upload button handler.
        uploadFile() {
            let input = document.createElement('input');
            input.type = 'file';
            input.accept = 'image/*';
            input.onchange = e => {
                let file = e.target.files[0];
                if (file.size > FileUploadMaxSize) {
                    this.ChatClient(`Please share an image smaller than ${FileUploadMaxSize / 1024 / 1024} MB in size!`);
                    return;
                }

                this.ChatClient(`<em>Uploading file to chat: ${file.name} - ${file.size} bytes, ${file.type} format.</em>`);

                // Get image file data.
                let reader = new FileReader();
                let rawData = new ArrayBuffer();
                reader.onload = e => {
                    rawData = e.target.result;

                    let fileByteArray = [],
                        u8array = new Uint8Array(rawData);
                    for (let i = 0; i < u8array.length; i++) {
                        fileByteArray.push(u8array[i]);
                    }

                    let msg = JSON.stringify({
                        action: "file",
                        channel: this.channel,
                        message: file.name,
                        bytes: fileByteArray, //btoa(fileByteArray),
                    });

                    // Send it to the chat server.
                    this.ws.conn.send(msg);
                };

                reader.readAsArrayBuffer(file);
            };
            input.click();
        },

        /**
         * Sound effect concerns.
         */

        setupSounds() {
            // Note: setupSounds had to be called on a page gesture (mouse click) before browsers
            // allow it to set up the AudioContext. If we've successfully set one up before, exit
            // this function immediately.
            if (this.config.sounds.audioContext) {
                if (this.config.sounds.audioContext.state === 'suspended') {
                    this.config.sounds.audioContext.resume();
                }
                return;
            }

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

        // Make all links in chat open in new windows
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

        /*
         * Idle Detection methods
         */

        setupIdleDetection() {
            window.addEventListener("keypress", this.deidle);
            window.addEventListener("mousemove", this.deidle);
        },

        // Common "de-idle" event handler
        deidle(e) {
            if (this.status === "idle") {
                this.status = "online";
            }

            if (this.idleTimeout !== null) {
                clearTimeout(this.idleTimeout);
            }

            this.idleTimeout = setTimeout(this.goIdle, 1000 * this.idleThreshold);
        },
        goIdle() {
            // only if we aren't already set on away
            if (this.status === "online") {
                this.status = "idle";
            }
        }
    }
});

app.mount("#BareRTC-App");
