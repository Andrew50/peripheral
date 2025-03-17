import { privateRequest } from '$lib/core/backend';
import type { Note, NoteFilter } from '$lib/core/types';

/**
 * Fetches all notes for the current user with optional filtering
 */
export async function getNotes(filter?: NoteFilter): Promise<Note[]> {
    return privateRequest<Note[]>('get_notes', filter || {});
}

/**
 * Fetches a single note by ID
 */
export async function getNote(noteId: number): Promise<Note> {
    return privateRequest<Note>('get_note', { noteId });
}

/**
 * Creates a new note
 */
export async function createNote(note: {
    title: string;
    content?: string;
    category?: string;
    tags?: string[];
    isPinned?: boolean;
    isArchived?: boolean;
}): Promise<Note> {
    return privateRequest<Note>('create_note', note);
}

/**
 * Updates an existing note
 */
export async function updateNote(note: {
    noteId: number;
    title: string;
    content?: string;
    category?: string;
    tags?: string[];
    isPinned?: boolean;
    isArchived?: boolean;
}): Promise<Note> {
    return privateRequest<Note>('update_note', note);
}

/**
 * Deletes a note
 */
export async function deleteNote(noteId: number): Promise<{ success: boolean; message: string }> {
    return privateRequest<{ success: boolean; message: string }>('delete_note', { noteId });
}

/**
 * Toggles the pinned status of a note
 */
export async function toggleNotePin(noteId: number, isPinned: boolean): Promise<Note> {
    return privateRequest<Note>('toggle_note_pin', { noteId, isPinned });
}

/**
 * Toggles the archived status of a note
 */
export async function toggleNoteArchive(noteId: number, isArchived: boolean): Promise<Note> {
    return privateRequest<Note>('toggle_note_archive', { noteId, isArchived });
} 