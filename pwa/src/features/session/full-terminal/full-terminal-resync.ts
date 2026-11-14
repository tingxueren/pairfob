/** Bound automatic view-only recovery; repeated gaps must not create a reconnect loop. */
export class FullTerminalResync {
  private owner = "";
  private started = 0;
  private attempts = 0;
  constructor(private readonly now = () => performance.now()) {}
  /** Closing and reopening only the view; the caller guards its own pane and renderer. */
  async recover(options: {
    owner: string;
    pause: () => Promise<void>;
    current: () => boolean;
    resume: () => Promise<void>;
  }): Promise<boolean> {
    if (!this.admit(options.owner)) {
      await Promise.resolve();
      return false;
    }
    await options.pause();
    if (options.current()) await options.resume();
    return true;
  }
  retry(busy: boolean, run: () => void): void {
    if (!busy) run();
  }
  admit(owner: string): boolean {
    const now = this.now();
    if (owner !== this.owner || now - this.started >= 30_000) {
      this.owner = owner;
      this.started = now;
      this.attempts = 0;
    }
    if (this.attempts >= 2) return false;
    this.attempts++;
    return true;
  }
}
