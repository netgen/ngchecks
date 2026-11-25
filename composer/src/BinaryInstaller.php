<?php

declare(strict_types=1);

namespace Netgen\Ngcheck\Composer;

use Composer\IO\IOInterface;
use Composer\Package\PackageInterface;
use Composer\Util\Filesystem;
use Composer\Util\HttpDownloader;
use PharData;
use RuntimeException;

use function bin2hex;
use function chmod;
use function explode;
use function file_get_contents;
use function file_put_contents;
use function getenv;
use function hash_equals;
use function hash_file;
use function is_executable;
use function is_file;
use function is_string;
use function ltrim;
use function php_uname;
use function preg_split;
use function random_bytes;
use function rename;
use function rtrim;
use function sprintf;
use function strtolower;
use function sys_get_temp_dir;
use function trim;

use const PHP_OS_FAMILY;

/**
 * Installs the ngcheck release archive (ngcheck + Betterleaks) for the current platform
 * into var/<platform>/ of the package, verified against the release's checksums.txt.
 */
final class BinaryInstaller
{
    private const DEFAULT_DOWNLOAD_URL = 'https://github.com/netgen/ngcheck/releases/download';

    private HttpDownloader $httpDownloader;

    private IOInterface $io;

    private string $packagePath;

    private Filesystem $filesystem;

    public function __construct(HttpDownloader $httpDownloader, IOInterface $io, string $packagePath)
    {
        $this->httpDownloader = $httpDownloader;
        $this->io = $io;
        $this->packagePath = $packagePath;
        $this->filesystem = new Filesystem();
    }

    public function install(PackageInterface $package): void
    {
        $platform = self::currentPlatform();

        if ($platform === null) {
            $this->io->writeError(sprintf('<warning>netgen/ngcheck: %s %s is not supported, vendor/bin/ngcheck will not work.</warning>', PHP_OS_FAMILY, php_uname('m')));

            return;
        }

        $version = $this->releaseVersion($package);

        if ($version === null) {
            return;
        }

        $directory = sprintf('%s/var/%s', $this->packagePath, $platform);
        $marker = $directory . '/.version';

        if (is_file($marker) && trim((string) file_get_contents($marker)) === $version && is_executable($directory . '/ngcheck')) {
            return;
        }

        $this->io->write(sprintf('<info>netgen/ngcheck: installing ngcheck %s (%s)</info>', $version, $platform));

        $archiveName = sprintf('ngcheck_%s_%s.tar.gz', $version, $platform);
        $baseUrl = sprintf('%s/v%s', rtrim(self::env('NGCHECK_DOWNLOAD_URL') ?? self::DEFAULT_DOWNLOAD_URL, '/'), $version);
        $workDirectory = sys_get_temp_dir() . '/netgen-ngcheck-' . bin2hex(random_bytes(8));

        try {
            $this->filesystem->ensureDirectoryExists($workDirectory);

            $expectedChecksum = $this->expectedChecksum($baseUrl . '/checksums.txt', $archiveName);

            $archive = $workDirectory . '/' . $archiveName;
            $this->httpDownloader->copy($baseUrl . '/' . $archiveName, $archive);

            $checksum = (string) hash_file('sha256', $archive);

            if (!hash_equals($expectedChecksum, $checksum)) {
                throw new RuntimeException(sprintf('netgen/ngcheck: checksum mismatch for %s: expected %s, got %s.', $archiveName, $expectedChecksum, $checksum));
            }

            $extracted = $workDirectory . '/extracted';
            (new PharData($archive))->extractTo($extracted, null, true);

            foreach (['ngcheck', 'betterleaks'] as $binary) {
                if (!is_file($extracted . '/' . $binary)) {
                    throw new RuntimeException(sprintf('netgen/ngcheck: %s does not contain %s.', $archiveName, $binary));
                }

                chmod($extracted . '/' . $binary, 0755);
            }

            file_put_contents($extracted . '/.version', $version . "\n");

            $this->filesystem->removeDirectory($directory);
            $this->filesystem->ensureDirectoryExists($this->packagePath . '/var');
            rename($extracted, $directory);
        } finally {
            $this->filesystem->removeDirectory($workDirectory);
        }
    }

    /**
     * The release to download: the installed version, or $NGCHECK_VERSION for development versions.
     */
    private function releaseVersion(PackageInterface $package): ?string
    {
        $override = self::env('NGCHECK_VERSION');

        if ($override !== null) {
            return ltrim($override, 'v');
        }

        if ($package->isDev()) {
            $this->io->writeError(sprintf(
                '<warning>netgen/ngcheck: %s is a development version and has no release binaries. '
                . 'Require a tagged release, or set NGCHECK_VERSION to the release to download.</warning>',
                $package->getPrettyVersion(),
            ));

            return null;
        }

        return ltrim($package->getPrettyVersion(), 'v');
    }

    private function expectedChecksum(string $checksumsUrl, string $archiveName): string
    {
        $checksums = (string) $this->httpDownloader->get($checksumsUrl)->getBody();

        foreach (preg_split('/\R/', $checksums) ?: [] as $line) {
            $parts = preg_split('/\s+/', trim($line)) ?: [];

            if (isset($parts[1]) && $parts[1] === $archiveName) {
                return strtolower($parts[0]);
            }
        }

        throw new RuntimeException(sprintf('netgen/ngcheck: %s is not listed in %s.', $archiveName, $checksumsUrl));
    }

    private static function currentPlatform(): ?string
    {
        switch (PHP_OS_FAMILY) {
            case 'Darwin':
                $os = 'darwin';

                break;

            case 'Linux':
                $os = 'linux';

                break;

            default:
                return null;
        }

        switch (strtolower(php_uname('m'))) {
            case 'x86_64':
            case 'amd64':
                return $os . '_x64';

            case 'arm64':
            case 'aarch64':
                return $os . '_arm64';

            default:
                return null;
        }
    }

    private static function env(string $name): ?string
    {
        $value = getenv($name);

        return is_string($value) && $value !== '' ? $value : null;
    }
}
