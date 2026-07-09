import { afterEach, describe, expect, it, vi } from "vitest";
import { connectWallet } from "./wallet";

describe("connectWallet", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("复用未完成的 MetaMask 连接请求，避免重复触发权限弹窗", async () => {
    let requestAccountsResolve: ((accounts: string[]) => void) | undefined;
    const request = vi.fn(({ method }: { method: string }) => {
      if (method === "eth_requestAccounts") {
        return new Promise<string[]>((resolve) => {
          requestAccountsResolve = resolve;
        });
      }
      if (method === "eth_chainId") {
        return "0xaa36a7";
      }
      throw new Error(`unexpected method: ${method}`);
    });

    vi.stubGlobal("window", { ethereum: { request } });

    const first = connectWallet();
    const second = connectWallet();
    await vi.waitFor(() => expect(request).toHaveBeenCalledWith({ method: "eth_requestAccounts", params: [] }));
    requestAccountsResolve?.(["0x1111111111111111111111111111111111111111"]);

    await expect(first).resolves.toMatchObject({ address: "0x1111111111111111111111111111111111111111" });
    await expect(second).resolves.toMatchObject({ address: "0x1111111111111111111111111111111111111111" });
    expect(request.mock.calls.filter(([payload]) => payload.method === "eth_requestAccounts")).toHaveLength(1);
  });

  it("把 MetaMask pending 权限错误转换成中文提示", async () => {
    const request = vi.fn(({ method }: { method: string }) => {
      if (method === "eth_requestAccounts") {
        throw {
          code: -32002,
          message: "Request of type 'wallet_requestPermissions' already pending.",
        };
      }
      throw new Error(`unexpected method: ${method}`);
    });

    vi.stubGlobal("window", { ethereum: { request } });

    await expect(connectWallet()).rejects.toThrow("MetaMask 已有连接请求待处理，请先在钱包弹窗中确认或取消。");
  });
});
