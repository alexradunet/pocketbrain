/**
 * Vault Provider
 * 
 * Singleton provider for vault service access.
 * Allows plugins to access the vault service without direct injection.
 */

import type { VaultService } from "./vault-service"

class VaultProvider {
  private vaultService: VaultService | null = null

  /**
   * Set the vault service instance
   */
  setVaultService(service: VaultService): void {
    this.vaultService = service
  }

  /**
   * Get the vault service instance
   */
  getVaultService(): VaultService | null {
    return this.vaultService
  }

  /**
   * Check if vault is enabled and initialized
   */
  isVaultEnabled(): boolean {
    return this.vaultService !== null
  }

  /**
   * Clear the vault service (for shutdown)
   */
  clear(): void {
    this.vaultService = null
  }
}

// Export singleton instance
export const vaultProvider = new VaultProvider()
