package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
//	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"log"
	"math/big"
	"net/http"
	"os"
	"bufio"
	"strings"
	"sync"
	"time"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"context"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"io/ioutil"
)

var (
	generatedPIN    []string
	mu              sync.Mutex
	rdb             *redis.Client
	ctx             = context.Background()
	fourLetterWords []string
)

func init() {
	// Initialize Redis client
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "password", // replace with your password if set
		DB:       0,  // use default DB
	})

	// Load a significantly large corpus of four-letter words
	loadFourLetterWordsFromFile("/usr/share/dict/words")
}

func loadFourLetterWordsFromFile(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal("Error opening wordlist file:", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := scanner.Text()
		// Filter out only valid four-letter words with alphabetical characters only
		if len(word) == 4 && isAlphabetic(word) {
			fourLetterWords = append(fourLetterWords, strings.ToUpper(word))
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal("Error reading wordlist file:", err)
	}

	log.Printf("Loaded %d four-letter words from the wordlist\n", len(fourLetterWords))
}

func isAlphabetic(s string) bool {
	for _, r := range s {
		if r < 'A' || (r > 'Z' && r < 'a') || r > 'z' {
			return false
		}
	}
	return true
}

func generatePIN() []string {
	// Generate a random 4-digit number for the first segment
	firstNumber, _ := rand.Int(rand.Reader, big.NewInt(9000))
	firstNumberStr := fmt.Sprintf("%04d", 1000+firstNumber.Int64())

	// Select a random word from the fourLetterWords list for the second segment
	wordIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(fourLetterWords))))
	word := fourLetterWords[wordIndex.Int64()]

	// Generate another random 4-digit number for the third segment
	secondNumber, _ := rand.Int(rand.Reader, big.NewInt(9000))
	secondNumberStr := fmt.Sprintf("%04d", 1000+secondNumber.Int64())

	// Return the final PIN segments as an array
	return []string{firstNumberStr, word, secondNumberStr}
}

func hashPIN(pin string) string {
	hasher := sha256.New()
	hasher.Write([]byte(pin))
	return base64.StdEncoding.EncodeToString(hasher.Sum(nil))
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	generatedPIN = generatePIN()
	browserUUID := uuid.New().String() // Generate UUID for this session

	// Concatenate the generated PIN segments to create the full PIN
	fullPIN := fmt.Sprintf("%s %s %s", generatedPIN[0], generatedPIN[1], generatedPIN[2])

	// Hash the generated PIN
	hashedPIN := hashPIN(fullPIN)

	// Store hashed PIN in Redis with the UUID as the key, with a TTL of 1 minute
	err := rdb.Set(ctx, browserUUID, hashedPIN, 1*time.Minute).Err()
	if err != nil {
		log.Fatal("Error storing PIN hash in Redis:", err)
	}
	log.Println("Stored hashed PIN in Redis with UUID:", browserUUID)
	mu.Unlock()

	tmpl, err := template.New("auth").Parse(`
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Human Authentication</title>
		<script src="/static/js/fp.min.js"></script>
		<style>
		@font-face {
			font-family: 'Dyslexie';
			src: url('/static/fonts/dyslexie.ttf') format('truetype');
		}
		body {
			font-family: 'DejaVu Sans Mono', Arial, sans-serif;
			src: url('/static/fonts/hack.ttf') format('truetype');
			display: flex;
			justify-content: center;
			align-items: center;
			height: 100vh;
			margin: 0;
			background-color: #f0f0f0;
		}
		.auth-container {
			background: #fff;
			padding: 30px;
			border-radius: 10px;
			box-shadow: 0px 0px 15px rgba(0, 0, 0, 0.1);
			text-align: center;
		}
		.images {
			margin-bottom: 20px;
		}
		.images img {
			width: 120px;
			height: auto;
			margin: 0 20px;
		}
		.pin-display {
			margin-bottom: 30px;
		}
		input[type="text"] {
			font-size: 24px;
			padding: 15px;
			width: calc(100% - 30px);
			border: 2px solid #ddd;
			border-radius: 8px;
			box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.1);
			transition: border-color 0.3s, box-shadow 0.3s;
		}
		input[type="text"]:focus {
			border-color: #4CAF50;
			box-shadow: 0 0 5px rgba(76, 175, 80, 0.5);
			outline: none;
		}
		button {
			font-size: 20px;
			cursor: not-allowed;
			background: grey;
			color: #fff;
			border: none;
			border-radius: 5px;
			padding: 10px 20px;
			margin-top: 20px;
			transition: background 0.3s;
		}
		button.enabled {
			cursor: pointer;
			background: #4CAF50;
		}
		</style>
	</head>
	<body>
		<div class="auth-container">
			<div class="images">
				<img src="static/img/human-ok.png" alt="Human OK">
				<img src="static/img/no-toasters.webp" alt="No Toasters">
			</div>
			<h2>Please Authenticate</h2>
			<p>Enter the PIN below to proceed:</p>
			<div class="pin-display">
				<img src="/pin-image?segment=first" alt="First PIN Image">
				<img src="/pin-image?segment=word" alt="Word PIN Image">
				<img src="/pin-image?segment=last" alt="Last PIN Image">
			</div>
			<input type="text" id="pin-input" placeholder="Enter PIN here" autofocus>
			<button id="submit-button" disabled>Submit</button>
		</div>
		<script>
			document.getElementById('pin-input').focus();

			const submitButton = document.getElementById('submit-button');
			sessionStorage.setItem('potato1', "");

			// Store UUID in sessionStorage
			const uuid = "{{.UUID}}";
			sessionStorage.setItem('potato2', uuid);

			// Load FingerprintJS
			FingerprintJS.load().then(fp => {
				console.log("FingerprintJS loaded successfully.");
				
				// Get the visitor identifier when available
				fp.get().then(result => {
					const visitorId = result.visitorId;
					const entropyLevel = visitorId.length;
					console.log("Entropy Level:", entropyLevel);
					sessionStorage.setItem('entropyLevel', entropyLevel); // Store entropy level in session storage

					// Enable the submit button if entropy level is high enough
					if (entropyLevel >= 10) {
						submitButton.disabled = false;
						submitButton.classList.add('enabled');
						submitButton.style.cursor = 'pointer';
						console.log("Submit button enabled based on entropy level.");
					} else {
						console.log("Entropy level is too low, submit button remains disabled.");
					}
				}).catch(error => {
					console.error("Error getting FingerprintJS visitor ID:", error);
				});
			}).catch(error => {
				console.error("Error loading FingerprintJS:", error);
			});

			submitButton.addEventListener('click', function() {
				const userInput = document.getElementById('pin-input').value;
				if (userInput) {
					const entropyLevel = sessionStorage.getItem('entropyLevel'); // Get the entropy level stored earlier
					// Redirect to the secured page, passing the values as query parameters
					const potato2 = sessionStorage.getItem('potato2');
					window.location.href = "/secured?userInput=" + encodeURIComponent(userInput) + "&potato2=" + encodeURIComponent(potato2) + "&entropy=" + encodeURIComponent(entropyLevel);
				} else {
					alert('Incorrect PIN. Please try again.');
				}
			});

			document.getElementById('pin-input').addEventListener('keyup', function(event) {
				if (event.key === 'Enter') {
					event.preventDefault();
					submitButton.click();
				}
			});

			// Clear session storage on page unload
			window.addEventListener('beforeunload', function() {
				sessionStorage.clear();
			});
		</script>
	</body>
	</html>
	`)
	if err != nil {
		log.Fatal(err)
	}

	tmpl.Execute(w, struct {
		UUID string
	}{UUID: browserUUID})
}

func pinImageHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	pinParts := generatedPIN

	// Map segment names to indexes
	segmentMap := map[string]int{
		"first": 0,
		"word":  1,
		"last":  2,
	}

	// Get the requested segment from the query parameter
	segment := r.URL.Query().Get("segment")
	index, exists := segmentMap[segment]
	if !exists {
		http.Error(w, "Invalid segment requested", http.StatusBadRequest)
		return
	}

	displayText := pinParts[index]

	const frameCount = 15
	const frameWidth = 150
	const frameHeight = 100

	frames := make([]*image.Paletted, frameCount)
	delays := make([]int, frameCount)
	palette := []color.Color{color.White, color.Black}

	// Create frames with a scrolling effect
	for i := 0; i < frameCount; i++ {
		rect := image.Rect(0, 0, frameWidth, frameHeight)
		img := image.NewPaletted(rect, palette)
		draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

		if i < frameCount-5 {
			// Randomly generate segments for initial frames
			if segment == "word" {
				wordIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(fourLetterWords))))
				displayText = fourLetterWords[wordIndex.Int64()]
			} else {
				randNum, _ := rand.Int(rand.Reader, big.NewInt(10000))
				displayText = fmt.Sprintf("%04d", randNum.Int64())
			}
		} else {
			// Gradually settle on the actual PIN part in the last few frames
			displayText = pinParts[index]
		}

		addLabelToPaletted(img, 20, 60, displayText)
		frames[i] = img

		// Set delays: shorter delay for initial frames, longer for final frame
		if i < frameCount-5 {
			delays[i] = 10 // Short delay for initial frames (100 ms)
		} else if i == frameCount-1 {
			delays[i] = 900 // Longer delay for the final frame (9 seconds)
		} else {
			delays[i] = 100 // Intermediate frames delay (1 second)
		}
	}

	// Set headers to prevent caching of the image
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "Thu, 01 Jan 1970 00:00:00 GMT")

	// Encode the frames as an animated GIF
	gif.EncodeAll(w, &gif.GIF{
		Image: frames,
		Delay: delays,
	})
}


func addLabelToPaletted(img *image.Paletted, x, y int, label string) {
	col := color.Black
	point := fixed.Point26_6{fixed.Int26_6(x << 6), fixed.Int26_6(y << 6)}

	face := loadFontWithFallback(40)

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  point,
	}
	d.DrawString(label)
}

func loadFontWithFallback(size float64) font.Face {
	fontBytes, err := ioutil.ReadFile("./static/fonts/dyslexie.ttf") // Adjust to a known working font file
	if err != nil {
		log.Printf("Failed to load Dyslexie font: %v. Falling back to basic font.", err)
		return basicfont.Face7x13 // Use a basic built-in font as a fallback
	}

	dyslexieFont, err := opentype.Parse(fontBytes)
	if err != nil {
		log.Printf("Failed to parse Dyslexie font: %v. Falling back to basic font.", err)
		return basicfont.Face7x13 // Use a basic built-in font as a fallback
	}

	face, err := opentype.NewFace(dyslexieFont, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Printf("Failed to create font face: %v. Falling back to basic font.", err)
		return basicfont.Face7x13 // Use a basic built-in font as a fallback
	}

	return face
}

func securedHandler(w http.ResponseWriter, r *http.Request) {
	userInput := r.URL.Query().Get("userInput")
	potato2 := r.URL.Query().Get("potato2")
	entropyLevel := r.URL.Query().Get("entropy")

	log.Println("Received userInput:", userInput)
	log.Println("Received potato2:", potato2)
	log.Println("Received entropyLevel:", entropyLevel)

	if userInput == "" || potato2 == "" {
		log.Println("Missing userInput or potato2, redirecting to auth page.")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	storedHash, err := rdb.Get(ctx, potato2).Result()
	if err != nil {
		log.Println("Error retrieving PIN hash from Redis:", err)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Hash the user input
	userInputHash := hashPIN(userInput)

	log.Println("Stored hash retrieved from Redis:", storedHash)
	log.Println("Hashed user input:", userInputHash)

	// Compare hashed values
	if userInputHash != storedHash {
		log.Println("User input does not match stored PIN hash, redirecting to auth page.")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Delete the PIN after a successful match to treat it as a one-time password
	rdb.Del(ctx, potato2)

	log.Println("User input matches, access granted to secured page.")
	w.Write([]byte(fmt.Sprintf("<h1>Human Page</h1><p>You look pretty human to me. Your PIN was: %s and entropy level: %s</p>", userInput, entropyLevel)))
}

func main() {
	http.HandleFunc("/", authHandler)
	http.HandleFunc("/pin-image", pinImageHandler)
	http.HandleFunc("/secured", securedHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
