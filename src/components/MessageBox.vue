<script>
export default {
    props: {
        message: Object,    // chat Message object
        user: Object,       // User object of the Message author
        username: String,   // current username logged in
        websiteUrl: String, // Base URL to website (for profile/avatar URLs)
        isDnd: Boolean,     // user is not accepting DMs
        isMuted: Boolean,   // user is muted by current user
        reactions: Object,  // emoji reactions on the message
        reportEnabled: Boolean, // Report Message webhook is available
        position: Number,   // position of the message (0 to n), for the emoji menu to know which side to pop
        isDm: Boolean,      // is in a DM thread (hide DM buttons)
        isOp: Boolean,      // current user is Operator (always show takeback button)
        noButtons: Boolean, // hide all message buttons (e.g. for Report Modal)
    },
    data() {
        return {
            reactionsAvailable: [
                ['❤️', '👍', '😂', '😉', '😢', '😡', '🥰'],
                ['😘', '👎', '☹️', '😭', '🤔', '🙄', '🤩'],
                ['👋', '🔥', '😈', '🍑', '🍆', '💦', '🍌'],
                ['😋', '⭐', '😇', '😴', '😱', '👀', '🎃'],
                ['🤮', '🥳', '🙏', '🤦', '💩', '🤯', '💯'],
                ['😏', '🙈', '🙉', '🙊', '☀️', '🌈', '🎂'],
            ],
        };
    },
    computed: {
        profileURL() {
            if (this.user.profileURL) {
                return this.urlFor(this.user.profileURL);
            }
            return null;
        },
        avatarURL() {
            if (this.user.avatar) {
                return this.urlFor(this.user.avatar);
            }
            return null;
        },
        nickname() {
            if (this.user.nickname) {
                return this.user.nickname;
            }
            return this.user.username;
        },
        hasReactions() {
            return this.reactions != undefined && Object.keys(this.reactions).length > 0;
        },
    },
    methods: {
        signIn() {
            this.$emit('signIn', this.username);
        },

        openProfile() {
            let url = this.profileURL;
            if (url) {
                window.open(url);
            }
        },

        openDMs() {
            this.$emit('send-dm', {
                username: this.message.username,
            });
        },

        muteUser() {
            this.$emit('mute-user', this.message.username);
        },

        takeback() {
            this.$emit('takeback', this.message);
        },

        removeMessage() {
            this.$emit('remove', this.message);
        },

        reportMessage() {
            this.$emit('report', this.message);
        },

        sendReact(emoji) {
            this.$emit('react', this.message, emoji);
        },

        urlFor(url) {
            // Prepend the base websiteUrl if the given URL is relative.
            if (url.match(/^https?:/i)) {
                return url;
            }
            return this.websiteUrl.replace(/\/+$/, "") + url;
        },

        // Current user has reacted to the message.
        iReacted(emoji) {
            if (!this.hasReactions) return false;

            // test whether the current user has reacted
            if (this.reactions[emoji] != undefined) {
                for (let reactor of this.reactions[emoji]) {
                    if (reactor === this.username) {
                        return true;
                    }
                }
            }
            return false;
        },

        // TODO: DRY
        prettyDate(date) {
            if (date == undefined) return '';
            let hours = date.getHours(),
                minutes = String(date.getMinutes()).padStart(2, '0'),
                ampm = hours >= 11 ? "pm" : "am";

            let hour = hours % 12 || 12;
            return `${(hour)}:${minutes} ${ampm}`;
        },
    }
}
</script>

<template>
    <div class="box mb-2 px-4 pt-3 pb-1 position-relative">
        <div class="media mb-0">
            <div class="media-left">
                <a :href="profileURL"
                    @click.prevent="openProfile()"
                    :class="{ 'cursor-default': !profileURL }">
                    <figure class="image is-48x48">
                        <img v-if="message.isChatServer" src="/static/img/server.png">
                        <img v-else-if="message.isChatClient" src="/static/img/client.png">
                        <img v-else-if="avatarURL"
                            :src="avatarURL">
                        <img v-else src="/static/img/shy.png">
                    </figure>
                </a>
            </div>
            <div class="media-content">
                <div class="columns is-mobile pb-0">
                    <div class="column is-narrow pb-0">
                        <strong :class="{
                            'has-text-success is-dark': message.isChatServer,
                            'has-text-warning is-dark': message.isAdmin,
                            'has-text-danger': message.isChatClient
                        }">

                            <!-- User nickname/display name -->
                            {{ nickname }}
                        </strong>
                    </div>
                    <div class="column has-text-right pb-0">
                        <small class="has-text-grey is-size-7" :title="message.at">{{ prettyDate(message.at) }}</small>
                    </div>
                </div>

                <!-- User @username below it which may link to a profile URL if JWT -->
                <div class="columns is-mobile pt-0" v-if="(message.isChatClient || message.isChatServer)">
                    <div class="column is-narrow pt-0">
                        <small v-if="!(message.isChatClient || message.isChatServer)">
                            <a v-if="profileURL"
                                :href="profileURL" target="_blank"
                                class="has-text-grey">
                                @{{ message.username }}
                            </a>
                            <span v-else class="has-text-grey">@{{ message.username }}</span>
                        </small>
                        <small v-else class="has-text-grey">internal</small>
                    </div>
                </div>
                <div v-else class="columns is-mobile pt-0">
                    <div class="column is-narrow pt-0">
                        <small v-if="!(message.isChatClient || message.isChatServer)">
                            <a v-if="profileURL"
                                :href="profileURL" target="_blank"
                                class="has-text-grey">
                                @{{ message.username }}
                            </a>
                            <span v-else class="has-text-grey">@{{ message.username }}</span>
                        </small>
                        <small v-else class="has-text-grey">internal</small>
                    </div>

                    <div class="column is-narrow pl-1 pt-0"
                        v-if="!noButtons">
                        <!-- DMs button -->
                        <button type="button" v-if="!(message.username === username || isDm)"
                            class="button is-grey is-outlined is-small px-2"
                            @click="openDMs()"
                            :title="isDnd ? 'This person is not accepting new DMs' : 'Open a Direct Message (DM) thread'"
                            :disabled="isDnd">
                            <i class="fa fa-comment"></i>
                        </button>

                        <!-- Mute button -->
                        <button type="button" v-if="!(message.username === username)"
                            class="button is-grey is-outlined is-small px-2 ml-1"
                            @click="muteUser()" title="Mute user">
                            <i class="fa fa-comment-slash" :class="{
                                'has-text-success': isMuted,
                                'has-text-danger': !isMuted
                            }"></i>
                        </button>

                        <!-- Owner or admin: take back the message -->
                        <button type="button" v-if="message.username === username || isOp"
                            class="button is-grey is-outlined is-small px-2 ml-1"
                            title="Take back this message (delete it for everybody)"
                            @click="takeback()">
                            <i class="fa fa-rotate-left has-text-danger"></i>
                        </button>

                        <!-- Everyone else: can hide it locally -->
                        <button type="button" v-if="message.username !== username"
                            class="button is-grey is-outlined is-small px-2 ml-1"
                            title="Hide this message (delete it only for your view)"
                            @click="removeMessage()">
                            <i class="fa fa-trash"></i>
                        </button>
                    </div>
                </div>
            </div>
        </div>

        <!-- Report & Emoji buttons -->
        <div v-if="message.msgID && !noButtons" class="emoji-button columns is-mobile is-gapless mb-0">
            <!-- Report message button -->
            <div class="column" v-if="reportEnabled && message.username !== username">
                <button class="button is-small is-outlined mr-1" :class="{
                    'is-danger': !message.reported,
                    'has-text-grey': message.reported
                }" title="Report this message"
                    @click="reportMessage()">
                    <i class="fa fa-flag"></i>
                    <i class="fa fa-check ml-1" v-if="message.reported"></i>
                </button>
            </div>

            <!-- Emoji reactions menu -->
            <div class="column dropdown is-right" :class="{ 'is-up': position >= 2 }"
                onclick="this.classList.toggle('is-active')">
                <div class="dropdown-trigger">
                    <button class="button is-small px-2" aria-haspopup="true"
                        :aria-controls="`react-menu-${message.msgID}`">
                        <span>
                            <i class="fa fa-heart has-text-grey"></i>
                            <i class="fa fa-plus has-text-grey pl-1"></i>
                        </span>
                    </button>
                </div>
                <div class="dropdown-menu" :id="`react-menu-${message.msgID}`" role="menu">
                    <div class="dropdown-content p-0">
                        <!-- Iterate over reactions in rows of emojis-->
                        <div class="columns is-mobile ml-0 mb-2 mr-1"
                            v-for="row in reactionsAvailable">

                            <!-- Loop over the icons -->
                            <div class="column p-0 is-narrow" v-for="i in row">
                                <button type="button" class="button px-2 mt-1 ml-1 mr-0 mb-1"
                                    @click="sendReact(i)">
                                    {{ i }}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Message box -->
        <div class="content pl-5 pb-3 pt-1 mb-5">
            <em v-if="message.action === 'presence'">{{ message.message }}</em>
            <div v-else v-html="message.message"></div>

            <!-- Reactions so far? -->
            <div v-if="hasReactions" class="mt-1">
                <span v-for="(users, emoji) in reactions"
                    class="tag is-secondary mr-1 cursor-pointer"
                    :class="{ 'is-success is-light': iReacted(msg, emoji), 'is-secondary': !iReacted(msg, emoji) }"
                    :title="emoji + ' by: ' + users.join(', ')" @click="sendReact(emoji)">
                    {{ emoji }} <small class="ml-1">{{ users.length }}</small>
                </span>
            </div>
        </div>

    </div>
</template>

<style scoped>
</style>
