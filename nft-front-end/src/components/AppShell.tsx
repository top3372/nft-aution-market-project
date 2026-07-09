import { NavLink } from "react-router-dom";
import { WalletButton } from "./WalletButton";

/**
 * 应用外壳。
 *
 * 统一放置市场、创建、我的三个主业务入口和钱包按钮；页面内容通过 children 注入，
 * 保持路由页面只关心各自业务流程。
 */
export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="app-shell">
      <header className="topbar">
        <NavLink to="/" className="brand">
          NFT Auction
        </NavLink>
        <nav className="nav">
          <NavLink to="/">拍卖</NavLink>
          <NavLink to="/create">创建</NavLink>
          <NavLink to="/my">我的</NavLink>
        </nav>
        <WalletButton />
      </header>
      <main className="main">{children}</main>
    </div>
  );
}
