/**
 * Memory Repository Port
 * Defines the interface for memory persistence operations.
 * Following Dependency Inversion Principle - Core depends on abstraction.
 */

export interface MemoryEntry {
  id: number
  fact: string
  source?: string | null
}

export interface MemoryRepository {
  /**
   * Append a new fact to memory
   * @returns true if inserted, false if duplicate
   */
  append(fact: string, source?: string): boolean

  /**
   * Delete a memory entry by ID
   */
  delete(id: number): boolean

  /**
   * Update a memory entry
   * @returns true if updated, false if duplicate or not found
   */
  update(id: number, fact: string): boolean

  /**
   * Get all memory entries
   */
  getAll(): MemoryEntry[]
}
