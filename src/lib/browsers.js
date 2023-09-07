// Try and detect whether the user is on an Apple Safari browser, which has
// special nuances in their WebRTC video sharing support. This is intended to
// detect: iPads, iPhones, and Safari on macOS.
function isAppleWebkit() {
    // By User-Agent.
    if (/iPad|iPhone|iPod/.test(navigator.userAgent)) {
        return true;
    }

    // By (deprecated) navigator.platform.
    if (navigator.platform === 'iPad' || navigator.platform === 'iPhone' || navigator.platform === 'iPod') {
        return true;
    }

    return false;
}

export { isAppleWebkit };
