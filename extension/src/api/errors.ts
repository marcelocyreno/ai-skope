/**
 * ApiError carries the server's own error code and message, so the pane can
 * show something specific ("That path is outside every folder you have
 * allowed") instead of a status number.
 */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }

  /** The pairing is gone or was never valid: the pane must re-pair. */
  get needsPairing(): boolean {
    return this.status === 401 || this.code === "unauthorized";
  }

  /** The server refused a path; the message explains which rule applied. */
  get isAccessRefusal(): boolean {
    return ["folder_not_allowed", "path_denied", "no_folders"].includes(this.code);
  }
}

/** NetworkError means the server could not be reached at all. */
export class NetworkError extends Error {
  constructor(readonly url: string, cause?: unknown) {
    super("The AI Skope Server isn't reachable.");
    this.name = "NetworkError";
    this.cause = cause;
  }
}
