"use client";

import React, { useEffect, useState } from "react";
import { DashboardStatsData, fetchStats } from "../lib/stats";
import styles from "./DashboardStats.module.css";

export default function DashboardStats() {
  const [stats, setStats] = useState<DashboardStatsData | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let isMounted = true;
    
    const loadData = async () => {
      try {
        setLoading(true);
        const data = await fetchStats();

        if (isMounted) {
          setStats(data);
          setError(null);
        }
      } catch (err) {
        if (isMounted) {
          setError("Failed to load dashboard statistics. Please try again.");
          console.error(err);
        }
      } finally {
        if (isMounted) {
          setLoading(false);
        }
      }
    };

    loadData();

    return () => {
      isMounted = false;
    };
  }, []);

  if (loading) {
    return (
      <div className={styles.loadingContainer}>
        <div className={styles.spinner}></div>
        <p className={styles.loadingText}>Analyzing Network Traffic...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.errorContainer}>
        <div className={styles.errorIcon}>⚠️</div>
        <h3 className={styles.errorTitle}>Connection Lost</h3>
        <p className={styles.errorText}>{error}</p>
        <button className={styles.retryButton} onClick={() => window.location.reload()}>
          Retry Connection
        </button>
      </div>
    );
  }

  if (!stats) return null;

  // Shadow mode changes the meaning of every number below, so the
  // labels change with it. "Blocked: 500" when nothing was blocked
  // would be the worst thing this dashboard could say.
  const shadow = !stats.enforcing;

  return (
    <>
      <div className={shadow ? styles.statusShadow : styles.statusEnforcing} role="status">
        <span className={styles.statusDot}></span>
        {shadow ? "Shadow mode — not enforcing" : "Enforcing"}
      </div>

      {shadow && (
        <div className={styles.shadowBanner} role="status">
          <strong>Shadow mode — nothing is being blocked.</strong>
          <span>
            Every number below is what bot-shield <em>would</em> have done to
            your traffic. Your visitors are unaffected.
          </span>
        </div>
      )}

      <div className={styles.statsGrid}>
        <div className={`${styles.statCard} ${styles.totalCard}`}>
          <h3 className={styles.statLabel}>Total Requests</h3>
          <p className={styles.statValue}>{stats.total_requests.toLocaleString()}</p>
          <div className={styles.glowEffect}></div>
        </div>

        <div className={`${styles.statCard} ${styles.passedCard}`}>
          <h3 className={styles.statLabel}>{shadow ? "Would pass" : "Passed"}</h3>
          <p className={styles.statValue}>{stats.passed.toLocaleString()}</p>
          <div className={styles.glowEffect}></div>
        </div>

        <div className={`${styles.statCard} ${styles.challengedCard}`}>
          <h3 className={styles.statLabel}>{shadow ? "Would challenge" : "Challenged"}</h3>
          <p className={styles.statValue}>{stats.challenged.toLocaleString()}</p>
          <div className={styles.glowEffect}></div>
        </div>

        <div className={`${styles.statCard} ${styles.blockedCard}`}>
          <h3 className={styles.statLabel}>{shadow ? "Would block" : "Blocked"}</h3>
          <p className={styles.statValue}>{stats.blocked.toLocaleString()}</p>
          <div className={styles.glowEffect}></div>
        </div>
      </div>
    </>
  );
}
