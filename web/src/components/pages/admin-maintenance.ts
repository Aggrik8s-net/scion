/**
 * Copyright 2026 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/**
 * Admin Maintenance Operations page component
 *
 * Phase 1: Read-only display of maintenance operations and migrations.
 * Shows a checklist of one-time migrations and a list of routine operations.
 */

import { LitElement, html, css, nothing } from 'lit';
import { customElement, state } from 'lit/decorators.js';

import { apiFetch, extractApiError } from '../../client/api.js';

interface MaintenanceOperation {
  id: string;
  key: string;
  title: string;
  description: string;
  category: string;
  status: string;
  createdAt: string;
  startedAt?: string;
  completedAt?: string;
  startedBy?: string;
  result?: string;
}

interface MaintenanceRun {
  id: string;
  status: string;
  startedAt: string;
  completedAt?: string;
  startedBy?: string;
  result?: string;
}

interface MaintenanceOperationWithRun extends MaintenanceOperation {
  lastRun?: MaintenanceRun;
}

interface MaintenanceResponse {
  migrations: MaintenanceOperation[] | null;
  operations: MaintenanceOperationWithRun[] | null;
}

@customElement('scion-page-admin-maintenance')
export class ScionPageAdminMaintenance extends LitElement {
  @state()
  private loading = true;

  @state()
  private error: string | null = null;

  @state()
  private migrations: MaintenanceOperation[] = [];

  @state()
  private operations: MaintenanceOperationWithRun[] = [];

  static override styles = css`
    :host {
      display: block;
    }

    .header {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      margin-bottom: 2rem;
    }

    .header sl-icon {
      color: var(--scion-primary, #3b82f6);
      font-size: 1.5rem;
    }

    .header h1 {
      font-size: 1.5rem;
      font-weight: 700;
      color: var(--scion-text, #1e293b);
      margin: 0;
    }

    /* ── Sections ───────────────────────────────────────────────────── */

    .section {
      background: var(--scion-surface, #ffffff);
      border: 1px solid var(--scion-border, #e2e8f0);
      border-radius: var(--scion-radius-lg, 0.75rem);
      padding: 1.5rem;
      margin-bottom: 1.5rem;
    }

    .section-title {
      font-size: 1.125rem;
      font-weight: 600;
      color: var(--scion-text, #1e293b);
      margin: 0 0 0.25rem 0;
    }

    .section-description {
      font-size: 0.875rem;
      color: var(--scion-text-muted, #64748b);
      margin: 0 0 1rem 0;
    }

    /* ── Cards ───────────────────────────────────────────────────────── */

    .card-list {
      display: flex;
      flex-direction: column;
      gap: 1rem;
    }

    .card {
      border: 1px solid var(--scion-border, #e2e8f0);
      border-radius: var(--scion-radius, 0.5rem);
      padding: 1.25rem;
    }

    .card-header {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      margin-bottom: 0.5rem;
    }

    .card-header sl-icon {
      font-size: 1.25rem;
      flex-shrink: 0;
    }

    .card-header sl-icon.pending {
      color: var(--scion-text-muted, #64748b);
    }

    .card-header sl-icon.completed {
      color: var(--sl-color-success-600, #16a34a);
    }

    .card-header sl-icon.failed {
      color: var(--sl-color-danger-600, #dc2626);
    }

    .card-header sl-icon.running {
      color: var(--scion-primary, #3b82f6);
    }

    .card-title {
      font-size: 1rem;
      font-weight: 600;
      color: var(--scion-text, #1e293b);
      flex: 1;
    }

    .card-description {
      font-size: 0.875rem;
      color: var(--scion-text-muted, #64748b);
      line-height: 1.5;
      margin-bottom: 0.75rem;
    }

    .card-meta {
      display: flex;
      gap: 1.5rem;
      flex-wrap: wrap;
      font-size: 0.8125rem;
      color: var(--scion-text-muted, #64748b);
    }

    .card-meta span {
      display: inline-flex;
      align-items: center;
      gap: 0.25rem;
    }

    /* ── Status badges ──────────────────────────────────────────────── */

    .status-badge {
      display: inline-flex;
      align-items: center;
      padding: 0.125rem 0.5rem;
      border-radius: 9999px;
      font-size: 0.75rem;
      font-weight: 500;
    }

    .status-badge.pending {
      background: var(--sl-color-warning-100, #fef3c7);
      color: var(--sl-color-warning-700, #a16207);
    }

    .status-badge.completed {
      background: var(--sl-color-success-100, #dcfce7);
      color: var(--sl-color-success-700, #15803d);
    }

    .status-badge.failed {
      background: var(--sl-color-danger-100, #fee2e2);
      color: var(--sl-color-danger-700, #b91c1c);
    }

    .status-badge.running {
      background: var(--sl-color-primary-100, #dbeafe);
      color: var(--sl-color-primary-700, #1d4ed8);
    }

    /* ── Empty state ─────────────────────────────────────────────────── */

    .empty-inline {
      padding: 1.5rem;
      text-align: center;
      color: var(--scion-text-muted, #64748b);
      font-size: 0.875rem;
    }

    /* ── Loading / Error ─────────────────────────────────────────────── */

    .loading-state {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      padding: 4rem 2rem;
      color: var(--scion-text-muted, #64748b);
    }

    .loading-state sl-spinner {
      font-size: 2rem;
      margin-bottom: 1rem;
    }

    .error-state {
      text-align: center;
      padding: 3rem 2rem;
      background: var(--scion-surface, #ffffff);
      border: 1px solid var(--sl-color-danger-200, #fecaca);
      border-radius: var(--scion-radius-lg, 0.75rem);
    }

    .error-state sl-icon {
      font-size: 3rem;
      color: var(--sl-color-danger-500, #ef4444);
      margin-bottom: 1rem;
    }

    .error-state h2 {
      font-size: 1.25rem;
      font-weight: 600;
      color: var(--scion-text, #1e293b);
      margin: 0 0 0.5rem 0;
    }

    .error-state p {
      color: var(--scion-text-muted, #64748b);
      margin: 0 0 1rem 0;
    }

    .error-details {
      font-family: var(--scion-font-mono, monospace);
      font-size: 0.875rem;
      background: var(--scion-bg-subtle, #f1f5f9);
      padding: 0.75rem 1rem;
      border-radius: var(--scion-radius, 0.5rem);
      color: var(--sl-color-danger-700, #b91c1c);
      margin-bottom: 1rem;
    }
  `;

  override connectedCallback(): void {
    super.connectedCallback();
    void this.loadData();
  }

  private async loadData(): Promise<void> {
    this.loading = true;
    this.error = null;

    try {
      const response = await apiFetch('/api/v1/admin/maintenance/operations');
      if (!response.ok) {
        throw new Error(await extractApiError(response, `HTTP ${response.status}: ${response.statusText}`));
      }

      const data = (await response.json()) as MaintenanceResponse;
      this.migrations = data.migrations ?? [];
      this.operations = data.operations ?? [];
    } catch (err) {
      console.error('Failed to load maintenance operations:', err);
      this.error = err instanceof Error ? err.message : 'Failed to load maintenance operations';
    } finally {
      this.loading = false;
    }
  }

  private formatDate(dateString: string | undefined): string {
    if (!dateString) return '';
    try {
      const date = new Date(dateString);
      if (isNaN(date.getTime())) return '';
      return date.toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
      });
    } catch {
      return dateString;
    }
  }

  private formatRelativeTime(dateString: string | undefined): string {
    if (!dateString) return '';
    try {
      const date = new Date(dateString);
      if (isNaN(date.getTime())) return '';
      const diffMs = Date.now() - date.getTime();
      const diffSeconds = Math.round(diffMs / 1000);
      const diffMinutes = Math.round(diffMs / (1000 * 60));
      const diffHours = Math.round(diffMs / (1000 * 60 * 60));
      const diffDays = Math.round(diffMs / (1000 * 60 * 60 * 24));

      const rtf = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });

      if (Math.abs(diffSeconds) < 60) {
        return rtf.format(-diffSeconds, 'second');
      } else if (Math.abs(diffMinutes) < 60) {
        return rtf.format(-diffMinutes, 'minute');
      } else if (Math.abs(diffHours) < 24) {
        return rtf.format(-diffHours, 'hour');
      } else {
        return rtf.format(-diffDays, 'day');
      }
    } catch {
      return dateString;
    }
  }

  private statusIcon(status: string): string {
    switch (status) {
      case 'completed':
        return 'check-circle-fill';
      case 'failed':
        return 'exclamation-circle-fill';
      case 'running':
        return 'hourglass-split';
      default:
        return 'circle';
    }
  }

  override render() {
    return html`
      <div class="header">
        <sl-icon name="wrench-adjustable"></sl-icon>
        <h1>Maintenance</h1>
      </div>

      ${this.loading
        ? this.renderLoading()
        : this.error
          ? this.renderError()
          : this.renderContent()}
    `;
  }

  private renderLoading() {
    return html`
      <div class="loading-state">
        <sl-spinner></sl-spinner>
        <p>Loading maintenance operations...</p>
      </div>
    `;
  }

  private renderError() {
    return html`
      <div class="error-state">
        <sl-icon name="exclamation-triangle"></sl-icon>
        <h2>Failed to Load</h2>
        <p>There was a problem connecting to the API.</p>
        <div class="error-details">${this.error}</div>
        <sl-button variant="primary" @click=${() => this.loadData()}>
          <sl-icon slot="prefix" name="arrow-clockwise"></sl-icon>
          Retry
        </sl-button>
      </div>
    `;
  }

  private renderContent() {
    return html`
      ${this.renderMigrations()}
      ${this.renderOperations()}
    `;
  }

  private renderMigrations() {
    return html`
      <div class="section">
        <h2 class="section-title">Migrations</h2>
        <p class="section-description">
          One-time data migrations that transition the system between states.
          Completed migrations cannot be re-run from the UI.
        </p>
        ${this.migrations.length === 0
          ? html`<div class="empty-inline">No migrations registered.</div>`
          : html`
              <div class="card-list">
                ${this.migrations.map((m) => this.renderMigrationCard(m))}
              </div>
            `}
      </div>
    `;
  }

  private renderMigrationCard(m: MaintenanceOperation) {
    return html`
      <div class="card">
        <div class="card-header">
          <sl-icon
            name="${this.statusIcon(m.status)}"
            class="${m.status}"
          ></sl-icon>
          <span class="card-title">${m.title}</span>
          <span class="status-badge ${m.status}">${m.status}</span>
        </div>
        <div class="card-description">${m.description}</div>
        <div class="card-meta">
          <span>Created: ${this.formatDate(m.createdAt)}</span>
          ${m.completedAt
            ? html`<span>Completed: ${this.formatDate(m.completedAt)}</span>`
            : nothing}
          ${m.startedBy
            ? html`<span>By: ${m.startedBy}</span>`
            : nothing}
        </div>
      </div>
    `;
  }

  private renderOperations() {
    return html`
      <div class="section">
        <h2 class="section-title">Routine Operations</h2>
        <p class="section-description">
          Repeatable infrastructure tasks. Execution will be available in a future update.
        </p>
        ${this.operations.length === 0
          ? html`<div class="empty-inline">No operations registered.</div>`
          : html`
              <div class="card-list">
                ${this.operations.map((op) => this.renderOperationCard(op))}
              </div>
            `}
      </div>
    `;
  }

  private renderOperationCard(op: MaintenanceOperationWithRun) {
    return html`
      <div class="card">
        <div class="card-header">
          <sl-icon name="play-circle" class="pending"></sl-icon>
          <span class="card-title">${op.title}</span>
        </div>
        <div class="card-description">${op.description}</div>
        ${op.lastRun
          ? html`
              <div class="card-meta">
                <span>
                  Last run: ${this.formatRelativeTime(op.lastRun.startedAt)}
                  (<span class="status-badge ${op.lastRun.status}">${op.lastRun.status}</span>)
                </span>
                ${op.lastRun.startedBy
                  ? html`<span>by ${op.lastRun.startedBy}</span>`
                  : nothing}
              </div>
            `
          : html`
              <div class="card-meta">
                <span>Never run</span>
              </div>
            `}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'scion-page-admin-maintenance': ScionPageAdminMaintenance;
  }
}
