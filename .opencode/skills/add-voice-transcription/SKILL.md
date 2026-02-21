---
name: add-voice-transcription
description: Add voice message transcription to PocketBrain using OpenAI's Whisper API. Automatically transcribes WhatsApp voice notes so the agent can read and respond to them.
---

# Add Voice Transcription

This skill adds automatic voice message transcription to PocketBrain's WhatsApp channel using OpenAI's Whisper API. When a voice note arrives, it is downloaded, transcribed, and delivered to the agent as `[Voice: <transcript>]`.

## Phase 1: Pre-flight

### Ask the user

1. **Do they have an OpenAI API key?** If yes, collect it now. If no, they'll need to create one at https://platform.openai.com/api-keys.

## Phase 2: Apply Code Changes

Read `src/channels/whatsapp.ts` to understand how messages are received, then apply these changes:

### Install dependency

```bash
bun add openai
```

### Create `src/transcription.ts`

```typescript
import OpenAI from 'openai';
import { logger } from './logger.js';

const client = process.env.OPENAI_API_KEY
  ? new OpenAI({ apiKey: process.env.OPENAI_API_KEY })
  : null;

/**
 * Transcribe an audio buffer using OpenAI Whisper.
 * Returns the transcript text, or null on failure.
 */
export async function transcribeAudio(
  audioBuffer: Buffer,
  mimeType: string = 'audio/ogg',
): Promise<string | null> {
  if (!client) {
    logger.warn('OPENAI_API_KEY not set — voice transcription disabled');
    return null;
  }
  try {
    const file = new File([audioBuffer], 'voice.ogg', { type: mimeType });
    const result = await client.audio.transcriptions.create({
      model: 'whisper-1',
      file,
    });
    logger.info({ chars: result.text.length }, 'Transcribed voice message');
    return result.text;
  } catch (err) {
    logger.error({ err }, 'OpenAI transcription failed');
    return null;
  }
}
```

### Modify `src/channels/whatsapp.ts`

Add voice message handling in the incoming message handler. Find where messages are received and formatted, then add:

```typescript
import { transcribeAudio } from '../transcription.js';

// In the message handler, detect voice/audio messages:
const isVoiceMessage =
  msg.message?.audioMessage?.ptt === true ||
  msg.message?.audioMessage !== undefined;

if (isVoiceMessage) {
  try {
    // Download the audio using downloadMediaMessage from baileys
    const buffer = await downloadMediaMessage(msg, 'buffer', {});
    const transcript = await transcribeAudio(buffer as Buffer);
    if (transcript) {
      messageText = `[Voice: ${transcript}]`;
    } else {
      messageText = '[Voice Message - transcription unavailable]';
    }
  } catch (err) {
    logger.error({ err }, 'Failed to download audio message');
    messageText = '[Voice Message - transcription failed]';
  }
}
```

Study the existing message handling code in `whatsapp.ts` to find the right place to inject this, following the patterns already used for other message types.

### Update `.env.example`

```bash
OPENAI_API_KEY=
```

### Validate changes

```bash
bun test
```

All existing tests must pass before proceeding.

## Phase 3: Configure

### Get OpenAI API key (if needed)

If the user doesn't have an API key:

> I need you to create an OpenAI API key:
>
> 1. Go to https://platform.openai.com/api-keys
> 2. Click "Create new secret key"
> 3. Give it a name (e.g., "PocketBrain Transcription")
> 4. Copy the key (starts with `sk-`)
>
> Cost: ~$0.006 per minute of audio (~$0.003 per typical 30-second voice note)

Wait for the user to provide the key.

### Add to environment

Add to `.env`:

```bash
OPENAI_API_KEY=<their-key>
```

### Build and restart

```bash
bun run docker:build
bun run docker:up
```

## Phase 4: Verify

### Test with a voice note

Tell the user:

> Send a voice note in any registered WhatsApp chat. The agent should receive it as `[Voice: <transcript>]` and respond to its content.

### Check logs if needed

```bash
bun run docker:logs
```

Look for:
- `Transcribed voice message` — successful transcription with character count
- `OPENAI_API_KEY not set` — key missing from `.env`
- `OpenAI transcription failed` — API error (check key validity, billing)
- `Failed to download audio message` — media download issue

## Troubleshooting

### Voice notes show "[Voice Message - transcription unavailable]"

1. Check `OPENAI_API_KEY` is set in `.env`
2. Verify key works: `curl -s https://api.openai.com/v1/models -H "Authorization: Bearer $OPENAI_API_KEY" | head -c 200`
3. Check OpenAI billing — Whisper requires a funded account

### Voice notes show "[Voice Message - transcription failed]"

Check logs for the specific error. Common causes:
- Network timeout — transient, will work on next message
- Invalid API key — regenerate at https://platform.openai.com/api-keys
- Rate limiting — wait and retry

### Agent doesn't respond to voice notes

Verify the chat is registered and the agent is running. Voice transcription only runs for registered groups.
