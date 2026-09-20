/**
 * The extension's own version, read from the one place that has to be right.
 *
 * Chrome refuses a package whose `manifest.json` version is not higher than the
 * published one, so that field is the version by definition. Anything typed
 * into the UI beside it is a copy that goes stale the first time a release
 * forgets it — which is what happened between 0.1.0 and 0.1.2.
 */
export const VERSION = chrome.runtime.getManifest().version;
