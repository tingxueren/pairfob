#![cfg(target_os = "linux")]

use std::fs::{self, File, OpenOptions};
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::time::{Duration, Instant};

use pairfob_desktop::process::{self, ProcessError};
use tempfile::TempDir;

fn held_executable(dir: &TempDir) -> (PathBuf, File) {
    let path = dir.path().join("pairfob");
    fs::write(&path, "#!/bin/sh\nprintf ready\n").unwrap();
    fs::set_permissions(&path, fs::Permissions::from_mode(0o755)).unwrap();
    let writer = OpenOptions::new().write(true).open(&path).unwrap();
    (path, writer)
}

#[test]
fn run_waits_for_a_busy_executable_without_replaying_a_command() {
    let dir = TempDir::new().unwrap();
    let (program, writer) = held_executable(&dir);
    let error = std::process::Command::new(&program).spawn().unwrap_err();
    assert_eq!(error.raw_os_error(), Some(26));
    let release = std::thread::spawn(move || {
        std::thread::sleep(Duration::from_millis(80));
        drop(writer);
    });
    let result = process::run(&program, &[], Duration::from_secs(2));
    release.join().unwrap();
    let output = result.expect("the executable becomes available");
    assert!(output.ok());
    assert_eq!(output.stdout, "ready");
}

#[test]
fn a_busy_executable_uses_the_command_timeout() {
    let dir = TempDir::new().unwrap();
    let (program, _writer) = held_executable(&dir);
    let started = Instant::now();
    let result = process::run(&program, &[], Duration::from_millis(40));
    assert!(matches!(result, Err(ProcessError::Timeout(_, _))));
    assert!(started.elapsed() < Duration::from_secs(1));
}

#[test]
fn pairing_uses_the_same_busy_executable_recovery() {
    let dir = TempDir::new().unwrap();
    let (program, writer) = held_executable(&dir);
    let release = std::thread::spawn(move || {
        std::thread::sleep(Duration::from_millis(80));
        drop(writer);
    });
    let result = pairfob_desktop::pairing::OfferSession::start(&program);
    release.join().unwrap();
    result
        .expect("pairing starts after the writer closes")
        .cancel();
}
