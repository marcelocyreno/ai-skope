/**
 * The extension's own version, read from the one place that has to be right.
 *
 * Chrome refuses a package whose `manifest.json` version is not higher than the
 * published one, so that field is the version by definition. Anything typed
 * into the UI beside it is a copy that goes stale the first time a release
 * forgets it — which is what happened between 0.1.0 and 0.1.2.
 */
export const VERSION = chrome.runtime.getManifest().version;

/**
 * The server API this build speaks.
 *
 * `/v1/health` reports the server's own `apiVersion`; a server answering with
 * a different one is speaking shapes this code was never written against, so
 * the pane refuses the turn up front instead of failing part-way through it,
 * on a response it cannot parse, with the question already spent.
 *
 * It is a single number because the server's `APIVersion` is (see
 * `docs/PUBLISHING.md`); whether a build should declare a range it speaks is
 * a question for the first bump, and the comparison lives in one place.
 */
export const API_VERSION = 1;
