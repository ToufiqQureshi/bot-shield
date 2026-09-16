import styles from "./page.module.css";
import DashboardStats from "../components/DashboardStats";

export default function Home() {
  return (
    <div className={styles.page}>
      <main className={styles.main}>
        <div className={styles.header}>
          <div className={styles.logo}>
            <span className={styles.shieldIcon}>🛡️</span>
            <h1>Bot-Shield</h1>
          </div>
        </div>

        <section className={styles.dashboardSection}>
          <div className={styles.sectionHeader}>
            <h2>Real-time Traffic Overview</h2>
            <p>Monitoring incoming network requests and bot activity.</p>
          </div>
          
          <DashboardStats />
        </section>
      </main>
    </div>
  );
}
