import Foundation
import Testing
@testable import OpenAcosmi

@Suite(.serialized)
struct OpenAcosmiConfigFileTests {
    @Test
    func configPathRespectsEnvOverride() async {
        let override = FileManager().temporaryDirectory
            .appendingPathComponent("openacosmi-config-\(UUID().uuidString)")
            .appendingPathComponent("openacosmi.json")
            .path

        await TestIsolation.withEnvValues(["OPENACOSMI_CONFIG_PATH": override]) {
            #expect(OpenAcosmiConfigFile.url().path == override)
        }
    }

    @MainActor
    @Test
    func remoteGatewayPortParsesAndMatchesHost() async {
        let override = FileManager().temporaryDirectory
            .appendingPathComponent("openacosmi-config-\(UUID().uuidString)")
            .appendingPathComponent("openacosmi.json")
            .path

        await TestIsolation.withEnvValues(["OPENACOSMI_CONFIG_PATH": override]) {
            OpenAcosmiConfigFile.saveDict([
                "gateway": [
                    "remote": [
                        "url": "ws://gateway.ts.net:19999",
                    ],
                ],
            ])
            #expect(OpenAcosmiConfigFile.remoteGatewayPort() == 19999)
            #expect(OpenAcosmiConfigFile.remoteGatewayPort(matchingHost: "gateway.ts.net") == 19999)
            #expect(OpenAcosmiConfigFile.remoteGatewayPort(matchingHost: "gateway") == 19999)
            #expect(OpenAcosmiConfigFile.remoteGatewayPort(matchingHost: "other.ts.net") == nil)
        }
    }

    @MainActor
    @Test
    func setRemoteGatewayUrlPreservesScheme() async {
        let override = FileManager().temporaryDirectory
            .appendingPathComponent("openacosmi-config-\(UUID().uuidString)")
            .appendingPathComponent("openacosmi.json")
            .path

        await TestIsolation.withEnvValues(["OPENACOSMI_CONFIG_PATH": override]) {
            OpenAcosmiConfigFile.saveDict([
                "gateway": [
                    "remote": [
                        "url": "wss://old-host:111",
                    ],
                ],
            ])
            OpenAcosmiConfigFile.setRemoteGatewayUrl(host: "new-host", port: 2222)
            let root = OpenAcosmiConfigFile.loadDict()
            let url = ((root["gateway"] as? [String: Any])?["remote"] as? [String: Any])?["url"] as? String
            #expect(url == "wss://new-host:2222")
        }
    }

    @Test
    func stateDirOverrideSetsConfigPath() async {
        let dir = FileManager().temporaryDirectory
            .appendingPathComponent("openacosmi-state-\(UUID().uuidString)", isDirectory: true)
            .path

        await TestIsolation.withEnvValues([
            "OPENACOSMI_CONFIG_PATH": nil,
            "OPENACOSMI_STATE_DIR": dir,
        ]) {
            #expect(OpenAcosmiConfigFile.stateDirURL().path == dir)
            #expect(OpenAcosmiConfigFile.url().path == "\(dir)/openacosmi.json")
        }
    }
}
