// Wails runtime bindings
import { OpenFile, SaveFile, Generate } from '../wailsjs/go/main/VocabApp';
// For model definitions, if needed. Usually, these are just strings.
// import * as models from '../../wailsjs/go/models';

// --- DOM Elements ---
const btnLoad = document.getElementById('btn-load');
const timerLabel = document.getElementById('timer-label');
const statusLabel = document.getElementById('status-label');
const comboModel = document.getElementById('combo-model');
const comboQType = document.getElementById('combo-q-type');
const sentenceCountFrame = document.getElementById('sentence-count-frame');
const spinSentenceCount = document.getElementById('spin-sentence-count');
const btnGenerate = document.getElementById('btn-generate');
const btnSave = document.getElementById('btn-save');
const textInput = document.getElementById('text-input');
const textOutput = document.getElementById('text-output');

// --- State ---
let timerInterval;
let startTime;
let loadedFilename = "result";

// --- Event Listeners ---

btnLoad.addEventListener('click', () => {
    OpenFile()
        .then(content => {
            textInput.value = content;
            // Extract filename from the path provided by the user
            // This is a simplification; Wails OpenFile returns content, not path directly
            // We will handle filename based on a mock or ask user
            statusLabel.textContent = "파일 로드 완료.";
            textOutput.value = "";
            btnSave.disabled = true;
        })
        .catch(err => {
            if (err) { // Wails returns an empty error on user cancel
               statusLabel.textContent = `오류: ${err}`;
            }
        });
});

btnGenerate.addEventListener('click', () => {
    const vocabBlock = textInput.value;
    if (!vocabBlock) {
        alert("먼저 TXT 파일을 불러오세요.");
        return;
    }

    // Model warnings
    const selectedModel = comboModel.options[comboModel.selectedIndex].text;
    let warningMessage = "";
    if (selectedModel === "GPT-5 pro") {
        warningMessage = "GPT-5 pro는 고성능 모델이므로, 비용이 많이 발생할 수 있습니다. 계속하시겠습니까?";
    } else if (selectedModel === "GPT-5 nano" || selectedModel === "GPT-4.1") {
        warningMessage = "성능이 낮은 모델이므로, 문제 생성 품질이 낮거나 오류가 발생할 수 있습니다. 계속하시겠습니까?";
    }

    if (warningMessage && !confirm(warningMessage)) {
        return;
    }

    // Sentence count validation
    let numSentences = parseInt(spinSentenceCount.value, 10);
    if (isNaN(numSentences) || numSentences < 1) {
        numSentences = 1;
    }

    if (comboQType.value === "빈칸 추론" && numSentences > 5) {
        if (!confirm("예문을 5개 이상 생성하면 API 비용이 증가할 수 있습니다. 계속하시겠습니까?")) {
            return;
        }
    }

    setUIState(false);
    startTimer();
    statusLabel.textContent = "생성 중...";

    Generate(vocabBlock, comboModel.value, comboQType.value, numSentences)
        .then(result => {
            stopTimer();
            textOutput.value = result;
            statusLabel.textContent = "생성 완료!";
            btnSave.disabled = false;
        })
        .catch(err => {
            stopTimer();
            statusLabel.textContent = `오류: ${err}`;
            alert(`생성 오류:\n${err}`);
        })
        .finally(() => {
            setUIState(true);
        });
});


btnSave.addEventListener('click', () => {
    const contentToSave = textOutput.value;
    if (!contentToSave) {
        alert("저장할 내용이 없습니다.");
        return;
    }

    const qTypeShortMap = {"빈칸 추론": "빈칸", "영영풀이": "영영", "뜻풀이 판단": "뜻풀이"};
    let qTypeShort = qTypeShortMap[comboQType.value] || "문제";
    
    // We don't have the original filename, so we'll use a default.
    // A better implementation might store the filename in a global variable after loading.
    const suggestedFilename = `${loadedFilename}_${qTypeShort}.txt`;
    
    SaveFile(contentToSave, suggestedFilename)
        .then(status => {
            statusLabel.textContent = status;
        })
        .catch(err => {
            statusLabel.textContent = `오류: ${err}`;
            alert(`저장 오류:\n${err}`);
        });
});

comboQType.addEventListener('change', () => {
    if (comboQType.value === "빈칸 추론") {
        sentenceCountFrame.style.display = 'flex';
    } else {
        sentenceCountFrame.style.display = 'none';
    }
});

// --- UI Helper Functions ---

function setUIState(enabled) {
    btnLoad.disabled = !enabled;
    btnGenerate.disabled = !enabled;
    btnSave.disabled = !enabled || !textOutput.value;
    comboModel.disabled = !enabled;
    comboQType.disabled = !enabled;
    spinSentenceCount.disabled = !enabled;
}

function startTimer() {
    startTime = Date.now();
    timerLabel.textContent = "0.0s";
    timerInterval = setInterval(() => {
        const elapsedTime = ((Date.now() - startTime) / 1000).toFixed(1);
        timerLabel.textContent = `${elapsedTime}s`;
    }, 100);
}

function stopTimer() {
    clearInterval(timerInterval);
    const finalTime = ((Date.now() - startTime) / 1000).toFixed(1);
    timerLabel.textContent = `완료: ${finalTime}s`;
}

// --- Initialisation ---
// Trigger change event to set initial visibility of sentence count
comboQType.dispatchEvent(new Event('change'));
// We need to call a startup function to get the initial filename if we were to implement that.
// For now, it's just basic setup.
console.log("Application started.");