//! A failed Linux exec may briefly see a writer on a newly replaced executable.
//! Retry only that pre-execution error; a started command is never replayed.

use std::io;
use std::process::{Child, Command};
use std::time::{Duration, Instant};

pub(crate) fn spawn(command: &mut Command) -> io::Result<Child> {
    spawn_until(command, Instant::now() + Duration::from_secs(1))
}

pub(crate) fn spawn_until(command: &mut Command, deadline: Instant) -> io::Result<Child> {
    loop {
        if Instant::now() >= deadline {
            return Err(io::Error::new(
                io::ErrorKind::TimedOut,
                "executable remained busy until the launch deadline",
            ));
        }
        match command.spawn() {
            // Linux overlayfs can retain a writer briefly after file creation.
            Err(error) if cfg!(target_os = "linux") && error.raw_os_error() == Some(26) => {
                std::thread::sleep(
                    Duration::from_millis(10)
                        .min(deadline.saturating_duration_since(Instant::now())),
                );
            }
            result => return result,
        }
    }
}
