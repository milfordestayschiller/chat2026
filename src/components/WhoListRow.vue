<script>
import VideoFlag from '../lib/VideoFlag';

export default {
    props: {
        user: Object,       // User object of the Message author
        username: String,   // current username logged in
        websiteUrl: String, // Base URL to website (for profile/avatar URLs)
        isDnd: Boolean,     // user is not accepting DMs
        isMuted: Boolean,   // user is muted by current user
        vipConfig: Object,  // VIP config settings for BareRTC
        isOp: Boolean,      // current user is operator (can always DM)
        isVideoNotAllowed: Boolean,  // whether opening this camera is not allowed
        videoIconClass: String,      // CSS class for the open video icon
        isWatchingTab: Boolean, // is the "Watching" tab (replace video button w/ boot)
    },
    data() {
        return {
            VideoFlag: VideoFlag,
        };
    },
    computed: {
        profileURL() {
            if (this.user.profileURL) {
                return this.urlFor(this.user.profileURL);
            }
            return null;
        },
        profileButtonClass() {
            // VIP background.
            let result = "";
            if (this.user.vip) {
                result = "has-background-vip ";
            }

            let gender = (this.user.gender || "").toLowerCase();
            if (gender.indexOf("m") === 0) {
                return result + "has-text-gender-male";
            } else if (gender.indexOf("f") === 0) {
                return result + "has-text-gender-female";
            } else if (gender.length > 0) {
                return result + "has-text-gender-other";
            }
            return "";
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
        openProfile() {
            let url = this.profileURL;
            if (url) {
                window.open(url);
            }
        },

        openDMs() {
            this.$emit('send-dm', {
                username: this.user.username,
            });
        },

        openVideo() {
            this.$emit('open-video', this.user);
        },

        muteUser() {
            this.$emit('mute-user', this.user.username);
        },

        // Boot user off your cam (for isWatchingTab)
        bootUser() {
            this.$emit('boot-user', this.user.username);
        },

        urlFor(url) {
            // Prepend the base websiteUrl if the given URL is relative.
            if (url.match(/^https?:/i)) {
                return url;
            }
            return this.websiteUrl.replace(/\/+$/, "") + url;
        },
    }
}
</script>

<template>
    <div class="columns is-mobile">
        <!-- Avatar URL if available -->
        <div class="column is-narrow pr-0" style="position: relative">
            <a :href="profileURL"
                @click.prevent="openProfile()"
                :class="{ 'cursor-default': !profileURL }" class="p-0">
                <img v-if="avatarURL" :src="avatarURL" width="24" height="24" alt="">
                <img v-else src="/static/img/shy.png" width="24" height="24">

                <!-- Away symbol -->
                <div v-if="user.status !== 'online'" class="status-away-icon">
                    <i v-if="user.status === 'away'" class="fa fa-clock has-text-light"
                        title="Status: Away"></i>
                    <i v-else-if="user.status === 'lunch'" class="fa fa-utensils has-text-light"
                        title="Status: Out to lunch"></i>
                    <i v-else-if="user.status === 'call'" class="fa fa-phone-volume has-text-light"
                        title="Status: On the phone"></i>
                    <i v-else-if="user.status === 'brb'" class="fa fa-stopwatch-20 has-text-light"
                        title="Status: Be right back"></i>
                    <i v-else-if="user.status === 'busy'" class="fa fa-briefcase has-text-light"
                        title="Status: Working"></i>
                    <i v-else-if="user.status === 'book'" class="fa fa-book has-text-light"
                        title="Status: Studying"></i>
                    <i v-else-if="user.status === 'gaming'"
                        class="fa fa-gamepad who-status-wide-icon-2 has-text-light"
                        title="Status: Gaming"></i>
                    <i v-else-if="user.status === 'idle'" class="fa-regular fa-moon has-text-light"
                        title="Status: Idle"></i>
                    <i v-else-if="user.status === 'horny'" class="fa fa-fire has-text-light"
                        title="Status: Horny"></i>
                    <i v-else-if="user.status === 'chatty'" class="fa fa-comment has-text-light"
                        title="Status: Chatty and sociable"></i>
                    <i v-else-if="user.status === 'introverted'" class="fa fa-spoon has-text-light"
                        title="Status: Introverted and quiet"></i>
                    <i v-else-if="user.status === 'exhibitionist'"
                        class="fa-regular fa-eye who-status-wide-icon-1 has-text-light"
                        title="Status: Watch me"></i>
                    <i v-else class="fa fa-clock has-text-light" :title="'Status: ' + user.status"></i>
                </div>
            </a>
        </div>
        <div class="column pr-0 is-clipped" :class="{ 'pl-1': avatarURL }">
            <strong class="truncate-text-line is-size-7"
                @click="openProfile()"
                :class="{'cursor-pointer': profileURL}">
                {{ user.username }}
            </strong>
            <sup class="fa fa-peace has-text-warning-dark is-size-7 ml-1" v-if="user.op"
                title="Operator"></sup>
            <sup class="is-size-7 ml-1" :class="vipConfig.Icon" v-else-if="user.vip"
                :title="vipConfig.Name"></sup>
        </div>
        <div class="column is-narrow pl-0">
            <!-- Emoji icon -->
            <span v-if="user.emoji" class="pr-1 cursor-default" :title="user.emoji">
                {{ user.emoji.split(" ")[0] }}
            </span>

            <!-- Profile button -->
            <button type="button" v-if="profileURL" class="button is-small px-2 py-1"
                :class="profileButtonClass" @click="openProfile()"
                :title="'Open profile page' + (user.gender ? ` (gender: ${user.gender})` : '') + (user.vip ? ` (${vipConfig.Name})` : '')">
                <i class="fa fa-user"></i>
            </button>

            <!-- Unmute User button (if muted) -->
            <button type="button" v-if="isMuted" class="button is-small px-2 py-1"
                @click="muteUser()" title="This user is muted. Click to unmute them.">
                <i class="fa fa-comment-slash has-text-danger"></i>
            </button>

            <!-- DM button (if not muted) -->
            <button type="button" v-else class="button is-small px-2 py-1" @click="openDMs(u)"
                :disabled="user.username === username || (user.dnd && !isOp)"
                :title="user.dnd ? 'This person is not accepting new DMs' : 'Send a Direct Message'">
                <i class="fa" :class="{ 'fa-comment': !user.dnd, 'fa-comment-slash': user.dnd }"></i>
            </button>

            <!-- Video button (Who List tab) -->
            <button type="button" class="button is-small px-2 py-1"
                v-if="!isWatchingTab"
                :disabled="!(user.video & VideoFlag.Active)" :class="{
                    'is-danger is-outlined': (user.video & VideoFlag.Active) && (user.video & VideoFlag.NSFW),
                    'is-info is-outlined': (user.video & VideoFlag.Active) && !(user.video & VideoFlag.NSFW),
                    'cursor-notallowed': isVideoNotAllowed,
                }" :title="`Open video stream` +
(user.video & VideoFlag.MutualRequired ? '; mutual video sharing required' : '') +
(user.video & VideoFlag.MutualOpen ? '; will auto-open your video' : '')"
                @click="openVideo()">
                <i class="fa" :class="videoIconClass"></i>
            </button>

            <!-- Boot from Video button (Watching tab) -->
            <button v-else type="button" class="button is-small px-2 py-1"
                @click="bootUser()"
                title="Kick this person off your cam">
                <i class="fa fa-user-xmark has-text-danger"></i>
            </button>
        </div>
    </div>
</template>

<style scoped>
</style>
