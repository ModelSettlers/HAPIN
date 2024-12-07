# Human Authentication PIN

This repository provides an example Go web application that requires human authentication via a PIN split into three distinct parts:  
1. A 4-digit numeric code (numbers only)  
2. A four-letter word (from a loaded wordlist)  
3. Another 4-digit numeric code (numbers only)

Additionally, it uses [FingerprintJS](https://fingerprint.com) to measure browser entropy and dynamically enables the "Submit" button only after a sufficient entropy level is reached. This helps ensure that requests are coming from a legitimate browser environment rather than automated scripts.

## Features

- **PIN Segmentation**: The PIN is visually represented as three separate "casino-style" animated images, each independently rotating through random values before settling on the final number/word.
- **Hashed PIN Storage**: The final PIN is hashed and stored in a Redis database, treating it as a one-time password (OTP).
- **Browser Entropy Check**: Integrates `fp.min.js` (FingerprintJS) to enable the submit button only when the browser entropy level is high enough.
- **Dynamic Visuals**: Uses Go's `image` and `gif` packages to create dynamic, rotating GIFs that reveal the PIN segment by segment.

## Requirements

- **Go**: Version 1.18+ recommended.  
- **Redis**: A running Redis instance on `localhost:6379` with no password (or update the code accordingly if using a password).  
- **Fonts & Static Assets**:  
  - `static/fonts/dyslexie.ttf` (or a fallback font)  
  - `static/js/fp.min.js` for FingerprintJS  
  - `static/img/human-ok.png` and `static/img/no-toasters.webp` for the visuals on the auth page.
- **Wordlist**: A local wordlist file (e.g., `/usr/share/dict/words`) containing a wide range of words, ensuring that four-letter words are available for the second segment of the PIN.
