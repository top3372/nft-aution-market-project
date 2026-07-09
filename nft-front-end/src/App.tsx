import { Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { HomePage } from "./routes/HomePage";
import { AuctionDetailPage } from "./routes/AuctionDetailPage";
import { CreateAuctionPage } from "./routes/CreateAuctionPage";
import { MyPage } from "./routes/MyPage";

/**
 * 前端路由入口。
 *
 * 三条业务主线分别对应拍卖市场、拍卖详情、创建拍卖和“我的”资产页；AppShell 提供
 * 统一导航和钱包入口。
 */
export default function App() {
  return (
    <AppShell>
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/auctions/:auctionId" element={<AuctionDetailPage />} />
        <Route path="/create" element={<CreateAuctionPage />} />
        <Route path="/my" element={<MyPage />} />
      </Routes>
    </AppShell>
  );
}
