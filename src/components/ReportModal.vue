<script>
export default {
    props: {
        visible: Boolean,
        busy: Boolean,
        user: Object,
        message: Object,
    },
    data() {
        return {
            // Configuration
            reportClassifications: [
                "It's spam",
                "It's abusive (racist, homophobic, etc.)",
                "It's malicious (e.g. link to a malware website, phishing)",
                "It's illegal (e.g. controlled substances, violence)",
                "It's child abuse (CP, CSAM, pedophilia, etc.)",
                "Other (please describe)",
            ],

            // Our settings.
            classification: "It's spam",
            comment: "",
        };
    },
    methods: {
        accept() {
            this.$emit('accept', {
                classification: this.classification,
                comment: this.comment,
            });
        },
        cancel() {
            this.$emit('cancel');
        },
    }
}
</script>

<template>
    <!-- Report Modal -->
    <div class="modal" :class="{ 'is-active': visible }">
        <div class="modal-background"></div>
        <div class="modal-content">
            <div class="card">
                <header class="card-header has-background-warning">
                    <p class="card-header-title has-text-dark">Report a message</p>
                </header>
                <div class="card-content">

                    <!-- Message preview we are reporting on
                         TODO: make it DRY: style copied/referenced from chat history cards -->
                    <div class="box mb-2 px-4 pt-3 pb-1 position-relative">
                        <div class="media mb-0">
                            <div class="media-left">
                                <figure class="image is-48x48">
                                    <img v-if="user?.avatar"
                                        :src="user?.avatar">
                                    <img v-else src="/static/img/shy.png">
                                </figure>
                            </div>
                            <div class="media-content">
                                <div>
                                    <strong>
                                        <!-- User nickname/display name -->
                                        {{ user?.nickname }}
                                    </strong>
                                </div>

                                <!-- User @username below it which may link to a profile URL if JWT -->
                                <div>
                                    <small class="has-text-grey">
                                        @{{ message.username }}
                                    </small>
                                </div>
                            </div>
                        </div>

                        <!-- Message copy -->
                        <div class="content pl-5 py-3 mb-5 report-modal-message" v-html="message.message">
                        </div>
                    </div>

                    <div class="field mb-1">
                        <label class="label" for="classification">Report classification:</label>
                        <div class="select is-fullwidth">
                            <select id="classification" v-model="classification" :disabled="busy">
                                <option v-for="i in reportClassifications" :value="i">{{ i }}</option>
                            </select>
                        </div>
                    </div>

                    <div class="field">
                        <label class="label" for="reportComment">Comment:</label>
                        <textarea class="textarea" v-model="comment" :disabled="busy" cols="80"
                            rows="2" placeholder="Optional: describe the issue"></textarea>
                    </div>

                    <div class="field">
                        <div class="control has-text-centered">
                            <button type="button" class="button is-link mr-4" :disabled="busy"
                                @click="accept()">Submit report</button>
                            <button type="button" class="button" @click="cancel()">Cancel</button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<style scoped>
</style>
