//! SHM IPC — Zero-copy frame passing via POSIX shared memory.
//!
//! Uses `memmap2` for memory mapping and `libc` for POSIX named semaphores.
//! Frame header layout (32 bytes, little-endian):
//!   [width:4][height:4][channels:4][frame_no:8][timestamp:8][data_size:4]

use std::ffi::CString;
use std::os::unix::io::FromRawFd;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

use memmap2::MmapMut;

use crate::{ARGUS_ERR_INTERNAL, ARGUS_ERR_INVALID_PARAM, ARGUS_ERR_NULL_PTR, ARGUS_OK};

const HEADER_SIZE: usize = 32;

/// Opaque SHM writer handle passed across FFI boundary.
pub struct ShmWriter {
    _shm_name: CString,
    sem_w_name: CString,
    sem_r_name: CString,
    mmap: MmapMut,
    shm_fd: i32,
    shm_size: usize,
    sem_w: *mut libc::sem_t,
    sem_r: *mut libc::sem_t,
    frame_no: AtomicU64,
}

// SAFETY: ShmWriter is only accessed through FFI one call at a time (Go side
// holds a mutex). The raw pointers are POSIX semaphore handles.
unsafe impl Send for ShmWriter {}
unsafe impl Sync for ShmWriter {}

impl Drop for ShmWriter {
    fn drop(&mut self) {
        // SAFETY: Valid POSIX semaphore pointers from sem_open.
        unsafe {
            if !self.sem_w.is_null() {
                libc::sem_close(self.sem_w);
                libc::sem_unlink(self.sem_w_name.as_ptr());
            }
            if !self.sem_r.is_null() {
                libc::sem_close(self.sem_r);
                libc::sem_unlink(self.sem_r_name.as_ptr());
            }
            if self.shm_fd >= 0 {
                libc::close(self.shm_fd);
            }
            libc::shm_unlink(self._shm_name.as_ptr());
        }
    }
}

// ===== C ABI Exports =====

/// Create a shared memory writer for the given max frame dimensions.
///
/// # Safety
/// All string pointers must be valid, NUL-terminated C strings.
/// `out_handle` must be non-null.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_shm_create(
    shm_name: *const libc::c_char,
    sem_writer: *const libc::c_char,
    sem_reader: *const libc::c_char,
    max_width: i32,
    max_height: i32,
    channels: i32,
    out_handle: *mut *mut ShmWriter,
) -> i32 {
    if shm_name.is_null()
        || sem_writer.is_null()
        || sem_reader.is_null()
        || out_handle.is_null()
    {
        return ARGUS_ERR_NULL_PTR;
    }
    if max_width <= 0 || max_height <= 0 || channels <= 0 {
        return ARGUS_ERR_INVALID_PARAM;
    }

    let max_data =
        max_width as usize * max_height as usize * channels as usize;
    let shm_size = HEADER_SIZE + max_data;

    // SAFETY: Caller guarantees valid NUL-terminated strings.
    let shm_cname =
        unsafe { std::ffi::CStr::from_ptr(shm_name).to_owned() };
    let sem_w_cname =
        unsafe { std::ffi::CStr::from_ptr(sem_writer).to_owned() };
    let sem_r_cname =
        unsafe { std::ffi::CStr::from_ptr(sem_reader).to_owned() };

    // SAFETY: shm_open with valid C string and standard flags.
    let fd = unsafe {
        libc::shm_open(
            shm_cname.as_ptr(),
            libc::O_CREAT | libc::O_RDWR,
            0o666,
        )
    };
    if fd < 0 {
        eprintln!("[argus_shm] shm_open failed: errno={}", errno());
        return ARGUS_ERR_INTERNAL;
    }

    // SAFETY: ftruncate on valid fd.
    if unsafe { libc::ftruncate(fd, shm_size as libc::off_t) } < 0 {
        eprintln!("[argus_shm] ftruncate failed: errno={}", errno());
        unsafe { libc::close(fd); }
        return ARGUS_ERR_INTERNAL;
    }

    // Memory-map via memmap2. We create a File from the raw fd, map it,
    // then forget the File to avoid double-close.
    // SAFETY: fd is a valid file descriptor from shm_open.
    let file = unsafe { std::fs::File::from_raw_fd(fd) };
    let mmap = match unsafe { MmapMut::map_mut(&file) } {
        Ok(m) => m,
        Err(e) => {
            eprintln!("[argus_shm] mmap failed: {e}");
            drop(file); // closes fd
            return ARGUS_ERR_INTERNAL;
        }
    };
    // Prevent File::drop from closing fd — we manage it ourselves.
    std::mem::forget(file);

    // Writer semaphore (starts at 1 = unlocked).
    let sem_w = unsafe {
        libc::sem_open(sem_w_cname.as_ptr(), libc::O_CREAT, 0o666, 1)
    };
    if sem_w == libc::SEM_FAILED {
        eprintln!(
            "[argus_shm] sem_open(writer) failed: errno={}",
            errno()
        );
        unsafe { libc::close(fd); }
        return ARGUS_ERR_INTERNAL;
    }

    // Reader semaphore (starts at 0 = locked).
    let sem_r = unsafe {
        libc::sem_open(sem_r_cname.as_ptr(), libc::O_CREAT, 0o666, 0)
    };
    if sem_r == libc::SEM_FAILED {
        eprintln!(
            "[argus_shm] sem_open(reader) failed: errno={}",
            errno()
        );
        unsafe {
            libc::sem_close(sem_w);
            libc::close(fd);
        }
        return ARGUS_ERR_INTERNAL;
    }

    let writer = Box::new(ShmWriter {
        _shm_name: shm_cname,
        sem_w_name: sem_w_cname,
        sem_r_name: sem_r_cname,
        mmap,
        shm_fd: fd,
        shm_size,
        sem_w,
        sem_r,
        frame_no: AtomicU64::new(0),
    });

    // SAFETY: out_handle checked non-null above.
    unsafe { *out_handle = Box::into_raw(writer); }
    ARGUS_OK
}

/// Write a frame to shared memory.
///
/// # Safety
/// `handle` must be a valid pointer from `argus_shm_create`.
/// `pixels` must point to at least `pixels_len` bytes.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_shm_write_frame(
    handle: *mut ShmWriter,
    width: i32,
    height: i32,
    channels: i32,
    pixels: *const u8,
    pixels_len: usize,
) -> i32 {
    if handle.is_null() || pixels.is_null() {
        return ARGUS_ERR_NULL_PTR;
    }
    if width <= 0 || height <= 0 || channels <= 0 {
        return ARGUS_ERR_INVALID_PARAM;
    }

    let data_size =
        width as usize * height as usize * channels as usize;
    if pixels_len < data_size {
        return ARGUS_ERR_INVALID_PARAM;
    }

    // SAFETY: handle was created by argus_shm_create.
    let writer = unsafe { &mut *handle };

    if HEADER_SIZE + data_size > writer.shm_size {
        return ARGUS_ERR_INVALID_PARAM;
    }

    // Wait for writer semaphore (consumer finished reading).
    unsafe { libc::sem_wait(writer.sem_w); }

    // Increment frame counter.
    let frame_no =
        writer.frame_no.fetch_add(1, Ordering::Relaxed) + 1;
    let now_ns = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos() as u64;

    // Write header (32 bytes, little-endian).
    let buf = &mut writer.mmap[..];
    buf[0..4].copy_from_slice(&(width as u32).to_le_bytes());
    buf[4..8].copy_from_slice(&(height as u32).to_le_bytes());
    buf[8..12].copy_from_slice(&(channels as u32).to_le_bytes());
    buf[12..20].copy_from_slice(&frame_no.to_le_bytes());
    buf[20..28].copy_from_slice(&now_ns.to_le_bytes());
    buf[28..32].copy_from_slice(&(data_size as u32).to_le_bytes());

    // Write pixel data — direct memcpy into mmap.
    // SAFETY: pixels verified non-null, pixels_len >= data_size.
    let src = unsafe { std::slice::from_raw_parts(pixels, data_size) };
    buf[HEADER_SIZE..HEADER_SIZE + data_size].copy_from_slice(src);

    // Signal reader.
    unsafe { libc::sem_post(writer.sem_r); }

    crate::metrics::inc_shm_writes();
    ARGUS_OK
}

/// Destroy a shared memory writer, releasing all resources.
///
/// # Safety
/// `handle` must be a valid pointer from `argus_shm_create`, or null.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_shm_destroy(handle: *mut ShmWriter) {
    if handle.is_null() {
        return;
    }
    // SAFETY: allocated via Box::into_raw in argus_shm_create.
    unsafe { let _ = Box::from_raw(handle); }
}

/// Get the current frame number.
///
/// # Safety
/// `handle` and `out_frame_no` must be non-null.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_shm_frame_number(
    handle: *const ShmWriter,
    out_frame_no: *mut u64,
) -> i32 {
    if handle.is_null() || out_frame_no.is_null() {
        return ARGUS_ERR_NULL_PTR;
    }
    let writer = unsafe { &*handle };
    unsafe {
        *out_frame_no = writer.frame_no.load(Ordering::Relaxed);
    }
    ARGUS_OK
}

/// Cleanup residual SHM and semaphore resources.
///
/// # Safety
/// All string pointers must be valid, NUL-terminated C strings.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_shm_cleanup(
    shm_name: *const libc::c_char,
    sem_writer: *const libc::c_char,
    sem_reader: *const libc::c_char,
) {
    unsafe {
        if !shm_name.is_null() { libc::shm_unlink(shm_name); }
        if !sem_writer.is_null() { libc::sem_unlink(sem_writer); }
        if !sem_reader.is_null() { libc::sem_unlink(sem_reader); }
    }
}

/// Simulate a reader consuming a frame (for testing).
///
/// # Safety
/// `handle` must be a valid pointer from `argus_shm_create`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_shm_simulate_reader(
    handle: *mut ShmWriter,
) -> i32 {
    if handle.is_null() {
        return ARGUS_ERR_NULL_PTR;
    }
    let writer = unsafe { &*handle };
    unsafe {
        libc::sem_post(writer.sem_w);
        libc::sem_wait(writer.sem_r);
    }
    ARGUS_OK
}

/// Get current errno value (helper).
fn errno() -> i32 {
    std::io::Error::last_os_error()
        .raw_os_error()
        .unwrap_or(-1)
}
