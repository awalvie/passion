// @ts-nocheck: node runs this with `node --test`, and the client has no node types.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { plainText } from './text.ts';

test('plainText drops emphasis, code and link markers', () => {
	assert.equal(plainText('**Setup** - Stand tall'), 'Setup - Stand tall');
	assert.equal(plainText('a *slow* and __firm__ pull'), 'a slow and firm pull');
	assert.equal(plainText('use `20mm` or [this edge](https://x.y/e)'), 'use 20mm or this edge');
	assert.equal(plainText('## How to\n\nHang'), 'How to\n\nHang');
});

test('plainText keeps list markers and words with stars or underscores', () => {
	assert.equal(plainText('- one\n* two\n1. three'), '- one\n* two\n1. three');
	assert.equal(plainText('3 * 4 and snake_case_name'), '3 * 4 and snake_case_name');
});
