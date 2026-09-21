import { describe, it, expect } from 'vitest';
import { bulkUploadMessage, rejectedFilesMessage } from './uploadFeedback';

describe('upload feedback', () => {
	describe('bulkUploadMessage', () => {
		it('announces a finished batch', () => {
			const msg = bulkUploadMessage({ added: 12, failed: [], outcome: 'complete' });
			expect(msg).toEqual({ type: 'success', title: 'Added 12 photos' });
		});

		it('uses the singular for a single photo', () => {
			const msg = bulkUploadMessage({ added: 1, failed: [], outcome: 'complete' });
			expect(msg.title).toBe('Added 1 photo');
		});

		it('names the photos it could not read', () => {
			const msg = bulkUploadMessage({
				added: 2,
				failed: ['a.heic', 'b.heic'],
				outcome: 'complete'
			});
			expect(msg.type).toBe('warning');
			expect(msg.title).toBe('Added 2 photos');
			// Not "could not read": the list also holds photos the frame refused.
			expect(msg.description).toBe('Could not add a.heic, b.heic.');
		});

		it('names exactly as many as the limit allows without an "and more" tail', () => {
			const msg = bulkUploadMessage({ added: 0, failed: ['a', 'b', 'c'], outcome: 'complete' });
			expect(msg.description).toBe('Could not add a, b, c.');
		});

		it('abbreviates a long list of failures rather than naming all of them', () => {
			const msg = bulkUploadMessage({
				added: 0,
				failed: ['a', 'b', 'c', 'd', 'e'],
				outcome: 'complete'
			});
			expect(msg.description).toContain('a, b, c and 2 more');
		});

		it('reports a batch the person stopped', () => {
			const msg = bulkUploadMessage({ added: 4, failed: [], outcome: 'stopped' });
			expect(msg).toEqual({ type: 'info', title: 'Stopped. Added 4 photos' });
		});

		it('reports a frame that stopped answering, over any read failures', () => {
			const msg = bulkUploadMessage({ added: 7, failed: ['x.jpg'], outcome: 'unreachable' });
			expect(msg.type).toBe('error');
			expect(msg.title).toBe('The frame stopped accepting photos');
			expect(msg.description).toContain('7 photos');
		});
	});

	describe('rejectedFilesMessage', () => {
		it('says nothing was added when the selection is over the cap', () => {
			// Verified against the real file input: going over maxFiles rejects the
			// whole selection, it does not take the first 200.
			const msg = rejectedFilesMessage([{ errors: ['TOO_MANY_FILES'] }], 200);
			expect(msg.type).toBe('warning');
			expect(msg.title).toContain('200');
			expect(msg.description).toContain('Nothing was added');
			expect(msg.description).not.toContain('The rest');
		});

		it('explains a file that was not an image', () => {
			const msg = rejectedFilesMessage([{ errors: ['FILE_INVALID_TYPE'] }], 200);
			expect(msg.type).toBe('warning');
			expect(msg.title).toBe('Some files were left out');
			expect(msg.description).toContain('image');
		});

		it('leads with the cap when a selection trips both', () => {
			const msg = rejectedFilesMessage(
				[{ errors: ['FILE_INVALID_TYPE'] }, { errors: ['TOO_MANY_FILES'] }],
				200
			);
			expect(msg.title).toContain('200');
		});
	});
});
