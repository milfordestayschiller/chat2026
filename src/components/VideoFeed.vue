<script>
import Slider from 'vue3-slider';

export default {
    props: {
        localVideo: Boolean,  // is our local webcam (not other's camera)
        poppedOut: Boolean,   // Video is popped-out and draggable
        username: String,     // username related to this video
        isExplicit: Boolean,  // camera is marked Explicit
        isMuted: Boolean,     // camera is muted on our end
        isSourceMuted: Boolean, // camera is muted on the broadcaster's end
        isWatchingMe: Boolean, // other video is watching us back
        isFrozen: Boolean,     // video is detected as frozen
    },
    components: {
        Slider,
    },
    data() {
        return {
            // Volume slider
            volume: 100,

            // Volume change debounce
            volumeDebounce: null,

            // Mouse over status
            mouseOver: false,
        };
    },
    computed: {
        videoID() {
            return this.localVideo ? 'localVideo' : `videofeed-${this.username}`;
        },
    },
    methods: {
        closeVideo() {
            // Note: closeVideo only available for OTHER peoples cameras.
            // Closes the WebRTC connection as the offerer.
            this.$emit('close-video', this.username, 'offerer');
        },

        reopenVideo() {
            // Note: goes into openVideo(username, force)
            this.$emit('reopen-video', this.username, true);
        },

        // Toggle the Mute button
        muteVideo() {
            this.$emit('mute-video', this.username);
        },

        popoutVideo() {
            this.$emit('popout', this.username);
        },

        fullscreen() {
            let $elem = document.getElementById(this.videoID);
            if ($elem) {
                if ($elem.requestFullscreen) {
                    $elem.requestFullscreen();
                } else {
                    window.alert("Fullscreen not supported by your browser.");
                }
            }
        },

        volumeChanged() {
            if (this.volumeDebounce !== null) {
                clearTimeout(this.volumeDebounce);
            }
            this.volumeDebounce = setTimeout(() => {
                this.$emit('set-volume', this.username, this.volume);
            }, 200);
        },
    }
}
</script>

<template>
    <div class="feed" :class="{
        'popped-out': poppedOut,
        'popped-in': !poppedOut,
    }" @mouseover="mouseOver = true" @mouseleave="mouseOver = false">
        <video class="feed" :id="videoID" autoplay :muted="localVideo"></video>

        <!-- Caption -->
        <div class="caption" :class="{
            'has-text-camera-blue': !isExplicit,
            'has-text-camera-red': isExplicit,
        }">
            <i class="fa fa-microphone-slash mr-1 has-text-grey" v-if="isSourceMuted"></i>
            {{ username }}
            <i class="fa fa-people-arrows ml-1 has-text-grey is-size-7" :title="username + ' is watching your camera too'"
                v-if="isWatchingMe"></i>

            <!-- Frozen stream detection -->
            <a class="fa fa-mountain ml-1" href="#" v-if="!localVideo && isFrozen" style="color: #00FFFF"
                @click.prevent="reopenVideo()" title="Frozen video detected!"></a>
        </div>

        <!-- Close button (others' videos only) -->
        <div class="close" v-if="!localVideo">
            <a href="#" class="button is-small is-danger is-outlined px-2" title="Close video" @click.prevent="closeVideo()">
                <i class="fa fa-close"></i>
            </a>
        </div>

        <!-- Controls -->
        <div class="controls">
            <!-- Mute Button -->
            <button type="button" v-if="!isMuted" class="button is-small is-success is-outlined ml-1 px-2"
                @click="muteVideo()">
                <i class="fa" :class="{
                    'fa-microphone': localVideo,
                    'fa-volume-high': !localVideo
                }"></i>
            </button>
            <button type="button" v-else class="button is-small is-danger ml-1 px-2" @click="muteVideo()">
                <i class="fa" :class="{
                    'fa-microphone-slash': localVideo,
                    'fa-volume-xmark': !localVideo
                }"></i>
            </button>

            <!-- Pop-out Video -->
            <button type="button" class="button is-small is-light is-outlined p-2 ml-2" title="Pop out"
                @click="popoutVideo()">
                <i class="fa fa-up-right-from-square"></i>
            </button>

            <!-- Full screen -->
            <button type="button" class="button is-small is-light is-outlined p-2 ml-2" title="Go full screen"
                @click="fullscreen()">
                <i class="fa fa-expand"></i>
            </button>
        </div>

        <!-- Volume slider -->
        <div class="volume-slider" v-show="!localVideo && !isMuted && mouseOver">
            <Slider v-model="volume" color="#00FF00" track-color="#006600" :min="0" :max="100" :step="1" :height="7"
                orientation="vertical" @change="volumeChanged">

            </Slider>
        </div>
    </div>
</template>

<style scoped>
.volume-slider {
    position: absolute;
    left: 18px;
    top: 30px;
    bottom: 44px;
}
</style>
