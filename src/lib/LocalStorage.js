// All the distinct localStorage keys used.
const keys = {
    'fontSizeClass': String,        // Text magnification
    'videoScale': String,           // Video magnification (CSS classnames)
    'imageDisplaySetting': String,  // Show/hide/expand image preference
    'scrollback': Number,           // Scrollback buffer (int)
    'preferredDeviceNames': Object, // Webcam/mic device names (object, keys video,audio)

    // Webcam settings (booleans)
    'videoMutual': Boolean,
    'videoMutualOpen': Boolean,
    'videoAutoMute': Boolean,
    'videoVipOnly': Boolean,

    // Booleans
    'joinMessages': Boolean,
    'exitMessages': Boolean,
    'watchNotif': Boolean,
    'muteSounds': Boolean,
    'closeDMs': Boolean,      // close unsolicited DMs

    // Don't Show Again on NSFW modals.
    'skip-nsfw-modal': Boolean,
}

// UserSettings centralizes browser settings for the chat room.
class UserSettings {
    constructor() {
        // Recall stored settings. Only set the keys that were
        // found in localStorage on page load.
        for (let key of Object.keys(keys)) {
            if (localStorage[key] != undefined) {
                switch (keys[key]) {
                    case String:
                        this[key] = localStorage[key];
                    case Number:
                        this[key] = parseInt(localStorage[key]);
                    case Boolean:
                        this[key] = localStorage[key] === "true";
                    case Object:
                        this[key] = JSON.parse(localStorage[key]);
                }
            }
        }

        console.log("LocalStorage: Loaded settings", this);
    }

    // Return all of the current settings where the user had actually
    // left a preference on them (was in localStorage).
    getSettings() {
        let result = {};
        for (let key of Object.keys(keys)) {
            if (this[key] != undefined) {
                result[key] = this[key];
            }
        }
        return result;
    }

    // Get a value from localStorage, if set.
    get(key) {
        return this[key];
    }

    // Generic setter.
    set(key, value) {
        if (keys[key] == undefined) {
            throw `${key}: not a supported localStorage setting`;
        }

        localStorage[key] = JSON.stringify(value);
        this[key] = value;
    }
}

// LocalStorage is a global singleton to access and update user settings.
const LocalStorage = new UserSettings();
export default LocalStorage;
